package counter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

func CountUniqueIPs(filePath string) (uint64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	bitmap := make([]uint32, 1<<27)
	chunks := make(chan []byte, runtime.NumCPU()*2)

	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	for i := 0; i < workers; i++ {
		wg.Go(func() {
			for data := range chunks {
				processChunkAtomic(data, bitmap)
			}
		})
	}

	err = readChunksByLine(file, 64*1024*1024, chunks)
	if err != nil {
		return 0, fmt.Errorf("read chunks by line: %w", err)
	}

	close(chunks)
	wg.Wait()

	var total uint64
	for _, w := range bitmap {
		total += uint64(bits.OnesCount32(w))
	}
	return total, nil
}

func readChunksByLine(file *os.File, chunkSize int, out chan<- []byte) error {
	buf := make([]byte, chunkSize)
	var remain []byte
	for {
		n, err := file.Read(buf)
		data := remain
		data = append(data, buf[:n]...)

		last := bytes.LastIndexByte(data, '\n')
		if last == -1 {
			remain = data
		} else {
			next := last + 1
			out <- data[:next]

			remLen := len(data) - next
			if remLen > 0 {
				remain = data[next:]
			} else {
				remain = nil
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if len(remain) > 0 {
		out <- remain
	}
	return nil
}

func processChunkAtomic(data []byte, bitmap []uint32) {
	start := 0
	for i := range data {
		if data[i] == '\n' {
			line := data[start:i]
			start = i + 1

			ipv := parseIPv4Bytes3(line)
			setBitAtomic(bitmap, ipv)
		}
	}

	if start < len(data) {
		line := data[start:]
		ipv := parseIPv4Bytes3(line)
		setBitAtomic(bitmap, ipv)
	}
}

func parseIPv4Bytes3(b []byte) uint32 {
	var ip [4]byte
	var ipOffset int
	for _, c := range b {
		if c == '.' {
			ipOffset++
			continue
		}
		ip[ipOffset] = ip[ipOffset]*10 + (c - '0')
	}
	return binary.BigEndian.Uint32(ip[:])
}

func setBitAtomic(bitmap []uint32, ipv uint32) bool {
	wordIdx := int(ipv >> 5)
	bit := uint32(1) << (ipv & 31)
	// CAS loop
	for {
		old := atomic.LoadUint32(&bitmap[wordIdx])
		if old&bit != 0 {
			return false
		}
		if atomic.CompareAndSwapUint32(&bitmap[wordIdx], old, old|bit) {
			return true
		}
	}
}
