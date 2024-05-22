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
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	yaml "gopkg.in/yaml.v3"
)

type ForwardingSet struct {
	Rules []Rule `yaml:"rules"`
}

type Rule struct {
	Name      string   `yaml:"name"`
	From      string   `yaml:"from"`
	Algorithm string   `yaml:"algorithm"`
	To        []string `yaml:"to"`
}

func main() {
	var (
		file        string
		verbose     bool
		dialTimeout time.Duration
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `mixctl - TCP L4 load-balancer

GitHub: https://github.com/inlets/mixctl

Usage:

  mixctl -f rules.yaml

Example config file:

version: 0.1

rules:
- name: rpi-k3s
  from: 127.0.0.1:6443
  algorithm: round-robin
  to:
    - 192.168.1.19:6443
    - 192.168.1.21:6443
    - 192.168.1.20:6443

rules:
- name: remap-local-ssh-port
  from: 127.0.0.1:2222
  to:
    - 127.0.0.1:22

Flags:

`)

		flag.PrintDefaults()
	}

	flag.StringVar(&file, "f", "rules.yaml", "Job to run or leave blank for job.yaml in current directory")
	flag.BoolVar(&verbose, "v", false, "Verbose output for opened and closed connections")
	flag.DurationVar(&dialTimeout, "t", time.Millisecond*1500, "Dial timeout")
	flag.Parse()

	if len(file) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	set := ForwardingSet{}
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s %s\n\nRun mixctl --help for usage\n", file, err.Error())
		os.Exit(1)
	}
	if err = yaml.Unmarshal(data, &set); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing file %s %s\n", file, err.Error())
		os.Exit(1)
	}

	if len(set.Rules) == 0 {
		fmt.Fprintf(os.Stderr, "No rules found in file %s\n", file)
		os.Exit(1)
	}

	fmt.Printf("Starting mixctl by https://inlets.dev/\n\n")
	fmt.Printf("%d\n", verbose)

	wg := sync.WaitGroup{}
	wg.Add(len(set.Rules))
	for i, rule := range set.Rules {
		if rule.Algorithm == "" {
			set.Rules[i].Algorithm = "random" // default algorithm
		}
		fmt.Printf("Forwarding (%s) from: %s to: %s algorithm: %s\n", rule.Name, rule.From, rule.To, rule.Algorithm)
	}
	fmt.Println()

	for _, rule := range set.Rules {
		// Copy the value to avoid the loop variable being reused
		r := rule
		go func() {
			if err := forward(r, verbose, dialTimeout); err != nil {
				log.Printf("error forwarding %s", err.Error())
				os.Exit(1)
			}
			defer wg.Done()
		}()
	}

	wg.Wait()
}

func forward(rule Rule, verbose bool, dialTimeout time.Duration) error {
	fmt.Printf("Listening on: %s\n", rule.From)
	l, err := net.Listen("tcp", rule.From)
	if err != nil {
		return fmt.Errorf("error listening on %s %s", rule.From, err.Error())
	}

	defer l.Close()

	seed := time.Now().UnixNano()
	localRand := rand.New(rand.NewSource(seed))

	roundRobinIndex := 0

	connectedCounters := make([]atomic.Int32, len(rule.To))

	for {
		// accept a connection on the local port of the load balancer
		local, err := l.Accept()
		if err != nil {
			return fmt.Errorf("error accepting connection %s", err.Error())
		}

		// pick from the list of upstream servers according to algorithm
		var index int
		switch rule.Algorithm {
		case "round-robin":
			roundRobinIndex = (roundRobinIndex + 1) % len(rule.To)
			index = roundRobinIndex
		case "random":
			index = localRand.Intn(len(rule.To))
		case "least-connected":
			// find least connected counter
			minValue := connectedCounters[0].Load()
			minIndex := 0
			for i := range connectedCounters {
				counterValue := connectedCounters[i].Load()
				if counterValue < minValue {
					minValue = counterValue
					minIndex = i
				}
			}
			index = minIndex
		default:
			fmt.Errorf("invalid load balancing algorithm %s. defaulting to random.", rule.Algorithm)
			index = localRand.Intn(len(rule.To))
		}

		connectedCounters[index].Add(1)
		upstream := rule.To[index]

		// A separate Goroutine means the loop can accept another
		// incoming connection on the local address
		go connect(local, upstream, rule.From, verbose, dialTimeout, &connectedCounters[index])
	}
}

// connect dials the upstream address, then copies data
// between it and connection accepted on a local port
func connect(local net.Conn, upstreamAddr string, from string, verbose bool, dialTimeout time.Duration, connectCounter *atomic.Int32) {
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

	connectCounter.Add(-1) // decrease connected counter
}

// copy copies data between two connections using io.Copy
// and will exit when either connection is closed or runs
// into an error
func copy(ctx context.Context, from net.Conn, to net.Conn) error {

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
