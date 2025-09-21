package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"runtime/pprof"

	"github.com/bits-and-blooms/bitset"
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

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <filename>")
		return
	}
	filename := os.Args[1]
	fmt.Println("Unique IP's: ", UniqueIPs(filename))
}

func UniqueIPs(fileName string) int {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	bs := bitset.New(math.MaxUint32)
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := scanner.Text()
		ip := net.ParseIP(line).To4()
		if ip == nil {
			continue
		}

		v := binary.BigEndian.Uint32(ip.To4())
		bs.Set(uint(v))
	}
	return int(bs.Count())
}
