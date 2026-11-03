package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/alexproskurov/ip-addr-counter/counter"
)

func run() (err error) {
	profile := flag.String("cpuprofile", "", "write CPU profile to this file (optional)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-cpuprofile path] <atomic|bitset> <filename>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		return fmt.Errorf("expected method and filename")
	}

	var c counter.Counter
	switch flag.Arg(0) {
	case "atomic":
		c = &counter.AtomicCounter{}
	case "bitset":
		c = &counter.BitsetCounter{}
	default:
		return fmt.Errorf("unknown method %q", flag.Arg(0))
	}

	if *profile != "" {
		f, e := os.Create(*profile)
		if e != nil {
			return fmt.Errorf("create CPU profile: %w", e)
		}
		defer f.Close()
		if e := pprof.StartCPUProfile(f); e != nil {
			return fmt.Errorf("start CPU profile: %w", e)
		}
		defer pprof.StopCPUProfile()
	}

	count, e := c.CountUniqueIPs(flag.Arg(1))
	if e != nil {
		return e
	}
	fmt.Printf("Unique IPs: %d\n", count)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
