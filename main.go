package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"
	"strings"

	"golang.org/x/sync/errgroup"
	yaml "gopkg.in/yaml.v3"
)

type ForwardingSet struct {
	Rules []Rule `yaml:"rules"`
}

type Rule struct {
	Name string   `yaml:"name"`
	From string   `yaml:"from"`
	To   []string `yaml:"to"`
}

func main() {
	var (
		file        string
		verbose     bool
		dialTimeout time.Duration
	)

	flag.StringVar(&file, "f", "", "Job to run or leave blank for job.yaml in current directory")
	flag.BoolVar(&verbose, "v", true, "Verbose output for opened and closed connections")
	flag.DurationVar(&dialTimeout, "t", time.Millisecond*1500, "Dial timeout")
	flag.Parse()

	if len(file) == 0 {
		fmt.Fprintf(os.Stderr, "usage: mixctl -f rules.yaml\n")
		os.Exit(1)
	}

	set := ForwardingSet{}
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file %s %s", file, err.Error())
		os.Exit(1)
	}
	if err = yaml.Unmarshal(data, &set); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing file %s %s", file, err.Error())
		os.Exit(1)
	}

	if len(set.Rules) == 0 {
		fmt.Fprintf(os.Stderr, "no rules found in file %s", file)
		os.Exit(1)
	}

	fmt.Printf("Starting mixctl by https://inlets.dev/\n\n") 

	wg := sync.WaitGroup{}
	wg.Add(len(set.Rules))
	for _, rule := range set.Rules {
		fmt.Printf("Forward (%s) from: %s to: %s\n", rule.Name, rule.From, rule.To)
	}
	fmt.Println()

	for _, rule := range set.Rules {
		// Copy the value to avoid the loop variable being reused
		r := rule

		// Read destinations from a file
		_to := []string{}
		for _, to := range r.To {
			sb := strings.Split(to, ":")
			if len(sb) == 2 && sb[0] == "file" {
				filename := sb[1]
				data, err := os.ReadFile(filename)
				if err != nil {
					log.Printf("error reading the file %s", err.Error())
				} else {
					s := fmt.Sprintf("%s", data)
					lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
					for _, line := range(lines) {
						addr := strings.Trim(line)
						if addr != "" {
							_to = _to.append(line)
						}
					}
				}
			} else {
				_to = _to.append(to)
			}
		}

		go func() {
			if err := forward(r.Name, r.From, _to, verbose, dialTimeout); err != nil {
				log.Printf("error forwarding %s", err.Error())
				os.Exit(1)
			}
			defer wg.Done()
		}()
	}

	wg.Wait()
}

func forward(name, from string, to []string, verbose bool, dialTimeout time.Duration) error {
	seed := time.Now().UnixNano()
	rand.Seed(seed)

	fmt.Printf("Listening on: %s\n", from)
	l, err := net.Listen("tcp", from)
	if err != nil {
		return fmt.Errorf("error listening on %s %s", from, err.Error())
	}

	defer l.Close()

	for {
		// accept a connection on the local port of the load balancer
		local, err := l.Accept()
		if err != nil {
			return fmt.Errorf("error accepting connection %s", err.Error())
		}

		// pick randomly from the list of upstream servers
		// available
		index := rand.Intn(len(to))
		upstream := to[index]

		// A separate Goroutine means the loop can accept another
		// incoming connection on the local address
		go connect(local, upstream, from, verbose, dialTimeout)
	}
}

// connect dials the upstream address, then copies data
// between it and connection accepted on a local port
func connect(local net.Conn, upstreamAddr, from string, verbose bool, dialTimeout time.Duration) {
	defer local.Close()

	// If Dial is used on its own, then the timeout can be as long
	// as 2 minutes on MacOS for an unreachable host
	upstream, err := net.DialTimeout("tcp", upstreamAddr, dialTimeout)
	if err != nil {
		log.Printf("error dialing %s %s", upstreamAddr, err.Error())
		return
	}
	defer upstream.Close()

	if verbose {
		log.Printf("Connected %s => %s (%s)",
			from,
			upstream.RemoteAddr().String(),
			local.RemoteAddr().String())
	}

	ctx := context.Background()
	if err := copy(ctx, local, upstream); err != nil && err.Error() != "done" {
		log.Printf("error forwarding connection %s", err.Error())
	}

	if verbose {
		log.Printf("Closed %s => %s (%s)",
			from,
			upstream.RemoteAddr().String(),
			local.RemoteAddr().String())
	}
}

// copy copies data between two connections using io.Copy
// and will exit when either connection is closed or runs
// into an error
func copy(ctx context.Context, from, to net.Conn) error {

	ctx, cancel := context.WithCancel(ctx)
	errgrp, _ := errgroup.WithContext(ctx)
	errgrp.Go(func() error {
		io.Copy(from, to)
		cancel()

		return fmt.Errorf("done")
	})
	errgrp.Go(func() error {
		io.Copy(to, from)
		cancel()

		return fmt.Errorf("done")
	})
	errgrp.Go(func() error {
		<-ctx.Done()

		// This closes both ends of the connection as
		// soon as possible.
		from.Close()
		to.Close()
		return fmt.Errorf("done")
	})

	if err := errgrp.Wait(); err != nil {
		return err
	}

	return nil
}
