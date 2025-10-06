package counter

import (
	"bytes"
	"fmt"
	"io"
	"math/bits"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

type AtomicCounter struct{}

var _ Counter = (*AtomicCounter)(nil)

func (c *AtomicCounter) CountUniqueIPs(filePath string) (uint64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	bitmap := make([]atomic.Uint32, 1<<27)
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
	for i := range bitmap {
		total += uint64(bits.OnesCount32(bitmap[i].Load()))
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

func processChunkAtomic(data []byte, bitmap []atomic.Uint32) {
	start := 0
	for i := range data {
		if data[i] == '\n' {
			line := data[start:i]
			start = i + 1

			ipv := parseIPv4Bytes(line)
			setBitAtomic(bitmap, ipv)
		}
	}

	if start < len(data) {
		line := data[start:]
		ipv := parseIPv4Bytes(line)
		setBitAtomic(bitmap, ipv)
	}
}

func setBitAtomic(bitmap []atomic.Uint32, ipv uint32) bool {
	wordIdx := int(ipv >> 5)
	bit := uint32(1) << (ipv & 31)
	// CAS loop
	for {
		old := bitmap[wordIdx].Load()
		if old&bit != 0 {
			return false
		}
		if bitmap[wordIdx].CompareAndSwap(old, old|bit) {
			return true
		}
	}
}
