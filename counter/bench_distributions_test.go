package counter_test

import (
	"bufio"
	"os"
	"strconv"
	"testing"

	"github.com/alexproskurov/ip-addr-counter/counter"
)

const distributionBenchLines = 3_000_000

// BenchmarkIPDistributions compares the implementations on different IPv4
// distributions. Every address is generated locally, without third-party code.
// Run with: go test -run '^$' -bench '^BenchmarkIPDistributions' -benchmem ./counter
func BenchmarkIPDistributions(b *testing.B) {
	scenarios := []struct {
		name   string
		makeIP func(int) uint32
		unique uint64
	}{
		{
			name:   "repeated_16",
			makeIP: func(i int) uint32 { return 0x0a000000 | uint32(i&15) },
			unique: 16,
		},
		{
			name:   "sequential_unique",
			makeIP: func(i int) uint32 { return uint32(i) },
			unique: distributionBenchLines,
		},
		{
			name: "scattered_unique",
			// Multiplication by an odd number is bijective modulo 2^32.
			// Thus the first 3 million addresses are all distinct.
			makeIP: func(i int) uint32 { return uint32(i)*1664525 + 1013904223 },
			unique: distributionBenchLines,
		},
	}

	impls := []struct {
		name string
		c    counter.Counter
	}{
		{"atomic", &counter.AtomicCounter{}},
		{"bitset", &counter.BitsetCounter{}},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			path, size := writeIPDistribution(b, scenario.makeIP)
			for _, impl := range impls {
				b.Run(impl.name, func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(size)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						got, err := impl.c.CountUniqueIPs(path)
						if err != nil {
							b.Fatal(err)
						}
						if got != scenario.unique {
							b.Fatalf("got %d unique IPs, want %d", got, scenario.unique)
						}
					}
				})
			}
		})
	}
}

func writeIPDistribution(tb testing.TB, generate func(int) uint32) (string, int64) {
	tb.Helper()
	path := tb.TempDir() + "/addresses.txt"
	file, err := os.Create(path)
	if err != nil {
		tb.Fatal(err)
	}
	writer := bufio.NewWriterSize(file, 1<<20)
	var line [16]byte
	for i := 0; i < distributionBenchLines; i++ {
		ip := generate(i)
		dst := line[:0]
		for octet := 3; octet >= 0; octet-- {
			if octet != 3 {
				dst = append(dst, '.')
			}
			dst = strconv.AppendUint(dst, uint64(byte(ip>>uint(octet*8))), 10)
		}
		dst = append(dst, '\n')
		if _, err := writer.Write(dst); err != nil {
			tb.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		tb.Fatal(err)
	}
	if err := file.Close(); err != nil {
		tb.Fatal(err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		tb.Fatal(err)
	}
	return path, stat.Size()
}
