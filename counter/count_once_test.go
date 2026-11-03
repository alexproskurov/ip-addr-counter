package counter

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// Every distinct address contributes exactly once, even if different workers
// attempt to insert it concurrently. This checks the CAS-based count.
func TestAtomicCountExactlyOnce(t *testing.T) {
	const workers = 24
	bitmap := make([]atomic.Uint32, 2)
	data := []byte(strings.Repeat("0.0.0.0\n0.0.0.31\n0.0.0.32\n0.0.0.63\n", 100))
	totals := make([]uint64, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			totals[i] = processChunkAtomic(data, bitmap)
		}(i)
	}
	wg.Wait()
	var total uint64
	for _, n := range totals {
		total += n
	}
	if total != 4 {
		t.Fatalf("new unique IPs counted = %d, want 4", total)
	}
	if bitmap[0].Load() != 0x80000001 || bitmap[1].Load() != 0x80000001 {
		t.Fatalf("unexpected bitmap words: %08x %08x", bitmap[0].Load(), bitmap[1].Load())
	}
}
