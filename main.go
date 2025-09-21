package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"math"
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
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		ip := parseIPv4Bytes(scanner.Bytes())
		bs.Set(uint(ip))
	}
	return int(bs.Count())
}

func parseIPv4Bytes(input []byte) uint32 {
	var ip [4]byte
	var ipOffset int
	for _, c := range input {
		if c == '.' {
			ipOffset++
			continue
		}
		ip[ipOffset] = ip[ipOffset]*10 + (c - '0')
	}
	return binary.BigEndian.Uint32(ip[:])
}
