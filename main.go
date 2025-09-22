package main

import (
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"github.com/alexproskurov/ip-addr-counter/counter"
)

func main() {
	f, err := os.Create("cpuprof.prof")
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	defer f.Close() // error handling omitted for example
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	defer pprof.StopCPUProfile()

	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <atomic|bitset> <filename>")
		return
	}

	method := os.Args[1]
	filename := os.Args[2]

	var c counter.Counter
	switch method {
	case "atomic":
		c = &counter.AtomicCounter{}
	case "bitset":
		c = &counter.BitsetCounter{}
	default:
		log.Fatal("unknown method")
	}

	count, err := c.CountUniqueIPs(filename)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Unique IPs: %d\n", count)
}
