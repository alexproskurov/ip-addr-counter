package counter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

const (
	// A smaller chunk size limits outstanding memory even on many-core machines.
	atomicChunkSize = 4 << 20
	// The longest accepted line is 15 IPv4 characters + optional CR.
	maxIPv4LineSize = 16
)

type AtomicCounter struct{}

// ipChunk keeps the complete owned allocation even if the data starts mid-buffer.
type ipChunk struct {
	storage []byte
	data    []byte
}

var _ Counter = (*AtomicCounter)(nil)

// CountUniqueIPs counts valid IPv4 addresses using a shared atomic bitmap.
// Each worker owns its input buffer until it returns it to the pool.
func (c *AtomicCounter) CountUniqueIPs(filePath string) (uint64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	bitmap := make([]atomic.Uint32, 1<<27)
	workers := runtime.GOMAXPROCS(0)
	// A bounded worker pool avoids hundreds of outstanding buffers on hosts
	// where GOMAXPROCS is much larger than the effective CPU quota.
	if workers > 16 {
		workers = 16
	}
	if workers < 1 {
		workers = 1
	}
	chunks := make(chan ipChunk, workers)
	pool := &sync.Pool{New: func() any { return make([]byte, atomicChunkSize) }}

	var wg sync.WaitGroup
	workerTotals := make([]uint64, workers)
	for i := 0; i < workers; i++ {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			var workerTotal uint64
			for chunk := range chunks {
				workerTotal += processChunkAtomic(chunk.data, bitmap)
				pool.Put(chunk.storage)
			}
			workerTotals[idx] = workerTotal
		}()
	}

	readErr := readChunksByLine(file, atomicChunkSize, chunks, pool)
	close(chunks) // Always close and drain, even when reading fails.
	wg.Wait()
	if readErr != nil {
		return 0, fmt.Errorf("read chunks by line: %w", readErr)
	}

	// Successful CAS is the unique transition 0 -> 1 for an IP.
	// Sum worker-local counts to avoid sweeping the entire 512 MiB bitmap.
	var total uint64
	for _, count := range workerTotals {
		total += count
	}
	return total, nil
}

// readChunksByLine splits a reader into independent chunks at newline boundaries.
// A chunk is NOT reused until its worker returns the buffer to pool.
// Input lines exceeding the maximal IPv4 length are discarded (not counted).
// Buffers are always bounded by chunkSize; extremely long lines are skipped.
func readChunksByLine(r io.Reader, chunkSize int, out chan<- ipChunk, pool *sync.Pool) error {
	if chunkSize <= maxIPv4LineSize {
		return fmt.Errorf("chunk size must exceed %d", maxIPv4LineSize)
	}
	var carry [maxIPv4LineSize]byte
	carryLen := 0
	skipping := false
	noProgress := 0

	for {
		buf := pool.Get().([]byte)
		if len(buf) < chunkSize {
			pool.Put(buf)
			return fmt.Errorf("pooled buffer is smaller than chunk size")
		}
		copy(buf[:carryLen], carry[:carryLen])
		n, readErr := r.Read(buf[carryLen:chunkSize])
		if n == 0 && readErr == nil {
			pool.Put(buf)
			noProgress++
			if noProgress >= 100 {
				return io.ErrNoProgress
			}
			continue
		}
		noProgress = 0
		data := buf[:carryLen+n]
		carryLen = 0

		// An overlong line may span multiple chunks. Skip until its newline.
		// We never append it to an unbounded leftover buffer.
		if skipping {
			if i := bytes.IndexByte(data, '\n'); i >= 0 {
				data = data[i+1:]
				skipping = false
			} else {
				data = nil
			}
		}

		if len(data) > 0 {
			last := bytes.LastIndexByte(data, '\n')
			if last < 0 {
				if len(data) > maxIPv4LineSize {
					skipping = true
				} else {
					carryLen = copy(carry[:], data)
				}
				pool.Put(buf)
			} else {
				tail := data[last+1:]
				if len(tail) > maxIPv4LineSize {
					skipping = true
				} else {
					carryLen = copy(carry[:], tail)
				}
				out <- ipChunk{storage: buf, data: data[:last+1]}
			}
		} else {
			pool.Put(buf)
		}

		if readErr == io.EOF {
			if !skipping && carryLen > 0 {
				final := pool.Get().([]byte)
				copy(final, carry[:carryLen])
				out <- ipChunk{storage: final, data: final[:carryLen]}
			}
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func processChunkAtomic(data []byte, bitmap []atomic.Uint32) uint64 {
	var total uint64
	for len(data) > 0 {
		end := bytes.IndexByte(data, '\n')
		var line []byte
		if end == -1 {
			line = data
			data = nil
		} else {
			line = data[:end]
			data = data[end+1:]
		}
		if ip, ok := parseIPv4Strict(line); ok && setBitAtomic(bitmap, ip) {
			total++
		}
	}
	return total
}

// CAS avoids an unnecessary write when a frequently repeated address is set.
func setBitAtomic(bitmap []atomic.Uint32, ipv uint32) bool {
	wordIdx := int(ipv >> 5)
	bit := uint32(1) << (ipv & 31)
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
