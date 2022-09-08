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
		file string
	)

	flag.StringVar(&file, "f", "", "Job to run or leave blank for job.yaml in current directory")

	flag.Parse()

	set := ForwardingSet{}
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("error reading file %s %s", file, err.Error())
	}
	if err = yaml.Unmarshal(data, &set); err != nil {
		log.Fatalf("error parsing file %s %s", file, err.Error())
	}

	fmt.Printf("mixctl by inlets..\n")

	wg := sync.WaitGroup{}
	wg.Add(len(set.Rules))
	for _, f := range set.Rules {

		r := f
		go func(rule *Rule) {
			fmt.Printf("Forward (%s) from: %s to: %s\n", rule.Name, rule.From, rule.To)

			if err := forward(rule.Name, rule.From, rule.To); err != nil {
				log.Printf("error forwarding %s", err.Error())
				os.Exit(1)
			}

			defer wg.Done()
		}(&r)
	}
	wg.Wait()

}

func forward(name, from string, to []string) error {
	seed := time.Now().UnixNano()
	rand.Seed(seed)

	l, err := net.Listen("tcp", from)
	if err != nil {
		return fmt.Errorf("error listening on %s %s", from, err.Error())
	}
	log.Printf("listening on %s", from)
	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("error accepting connection %s", err.Error())
		}

		index := rand.Intn(len(to))

		remote, err := net.Dial("tcp", to[index])
		if err != nil {
			return fmt.Errorf("error dialing %s %s", to[index], err.Error())
		}

		go func() {
			log.Printf("[%s] %s => %s",
				from,
				conn.RemoteAddr().String(),
				remote.RemoteAddr().String())
			if err := forwardConnection(conn, remote); err != nil && err.Error() != "done" {
				log.Printf("error forwarding connection %s", err.Error())
			}
		}()
	}
}

func forwardConnection(from, to net.Conn) error {
	errgrp, _ := errgroup.WithContext(context.Background())
	errgrp.Go(func() error {
		io.Copy(from, to)

		return fmt.Errorf("done")
	})
	errgrp.Go(func() error {
		io.Copy(to, from)
		return fmt.Errorf("done")
	})

	return errgrp.Wait()
}
