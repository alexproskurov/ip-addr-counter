package counter

import (
	"bufio"
	"io"
	"os"
)

type BitsetCounter struct{}

var _ Counter = (*BitsetCounter)(nil)

const (
	// Each page represents one /16 IPv4 range: 65536 bits = 8 KiB.
	bitsetPageWords = 1024
	// Prefer a sparse table until it would use more than 512 KiB for pages.
	// Beyond this, switch to a single contiguous bitmap for random input.
	bitsetDenseThreshold = 64
)

// adaptiveBitmap starts sparse and switches to dense storage when addresses
// span many separate /16 ranges. This saves memory and initialization cost
// for clustered addresses without sacrificing random-address performance.
type adaptiveBitmap struct {
	pages     []*[bitsetPageWords]uint64
	dense     []uint64
	pageCount int
}

func newAdaptiveBitmap() *adaptiveBitmap {
	return &adaptiveBitmap{pages: make([]*[bitsetPageWords]uint64, 1<<16)}
}

// set returns true iff ip was not present. No external bitset dependency.
func (b *adaptiveBitmap) set(ip uint32) bool {
	if b.dense != nil {
		idx := ip >> 6
		mask := uint64(1) << (ip & 63)
		old := b.dense[idx]
		if old&mask != 0 {
			return false
		}
		b.dense[idx] = old | mask
		return true
	}

	pageIdx := ip >> 16
	page := b.pages[pageIdx]
	if page == nil {
		if b.pageCount == bitsetDenseThreshold {
			// Convert without losing any set bits or the current count.
			b.dense = make([]uint64, 1<<26)
			for i, src := range b.pages {
				if src != nil {
					copy(b.dense[i*bitsetPageWords:(i+1)*bitsetPageWords], src[:])
				}
			}
			b.pages = nil
			return b.set(ip)
		}
		page = new([bitsetPageWords]uint64)
		b.pages[pageIdx] = page
		b.pageCount++
	}
	idx := (ip >> 6) & (bitsetPageWords - 1)
	mask := uint64(1) << (ip & 63)
	old := page[idx]
	if old&mask != 0 {
		return false
	}
	page[idx] = old | mask
	return true
}

// CountUniqueIPs counts valid IPv4 addresses with a self-contained adaptive
// bitset. Invalid or overly long lines are ignored consistently with atomic.
func (c *BitsetCounter) CountUniqueIPs(filePath string) (uint64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	bitmap := newAdaptiveBitmap()
	var count uint64
	reader := bufio.NewReaderSize(file, 64*1024)
	for {
		line, err := reader.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			// No valid IPv4 string can exceed the reader buffer.
			for err == bufio.ErrBufferFull {
				_, err = reader.ReadSlice('\n')
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return 0, err
			}
			continue
		}
		if err != nil && err != io.EOF {
			return 0, err
		}
		if n := len(line); n > 0 && line[n-1] == '\n' {
			line = line[:n-1]
		}
		if ip, ok := parseIPv4Strict(line); ok && bitmap.set(ip) {
			count++
		}
		if err == io.EOF {
			break
		}
	}
	return count, nil
}
