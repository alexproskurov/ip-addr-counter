package counter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestParseIPv4Strict(t *testing.T) {
	tests := []struct {
		line string
		want uint32
		ok   bool
	}{
		{"0.0.0.0", 0, true},
		{"255.255.255.255", ^uint32(0), true},
		{"1.2.3.4", 0x01020304, true},
		{"1.2.3.4\r", 0x01020304, true},
		{"8.8.8.8", 0x08080808, true},
		{"", 0, false},
		{"\r", 0, false},
		{"abc", 0, false},
		{"256.1.1.1", 0, false},
		{"1.2.3", 0, false},
		{"1.2.3.4.5", 0, false},
		{"1.2..4", 0, false},
		{".1.2.3.4", 0, false},
		{"1.2.3.4.", 0, false},
		{"1.2.3.-4", 0, false},
		{"1.2.3.04", 0, false},
		{"001.2.3.4", 0, false},
		{"1.2.3.4x", 0, false},
		{"1.2.3.4\r\r", 0, false},
		{"999999999999.1.2.3", 0, false},
		{" 1.2.3.4", 0, false},
		{"1.2.3.4 ", 0, false},
		{"1.2.3.4\x00", 0, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.line), func(t *testing.T) {
			got, ok := parseIPv4Strict([]byte(tt.line))
			if got != tt.want || ok != tt.ok {
				t.Fatalf("parseIPv4Strict(%q) = (%08x, %v), want (%08x, %v)", tt.line, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func FuzzParseIPv4Strict(f *testing.F) {
	for _, seed := range []string{"0.0.0.0", "255.255.255.255", "1.2.3.4", "1.2.3.4.5", "256.1.1.1", "", "abc"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, line []byte) {
		_, _ = parseIPv4Strict(line) // Must never panic on arbitrary input.
	})
}

func testChunkedIPs(t *testing.T, src []byte, chunkSize int, want []uint32) {
	t.Helper()
	pool := &sync.Pool{New: func() any { return make([]byte, chunkSize) }}
	chunks := make(chan ipChunk, 2)
	var got []uint32
	done := make(chan struct{})
	go func() {
		defer close(done)
		for chunk := range chunks {
			for _, line := range bytes.Split(chunk.data, []byte{'\n'}) {
				if ip, ok := parseIPv4Strict(line); ok {
					got = append(got, ip)
				}
			}
			pool.Put(chunk.storage)
		}
	}()
	err := readChunksByLine(bytes.NewReader(src), chunkSize, chunks, pool)
	close(chunks)
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("chunkSize=%d: got %v, want %v", chunkSize, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("chunkSize=%d: got %v, want %v", chunkSize, got, want)
		}
	}
}

func TestChunkBoundaries(t *testing.T) {
	src := []byte("1.2.3.4\n255.255.255.255\r\nabc\n8.8.8.8\n10.0.0.1")
	want := []uint32{0x01020304, ^uint32(0), 0x08080808, 0x0a000001}
	for _, size := range []int{17, 18, 19, 21, 23, 31, 64, 128} {
		testChunkedIPs(t, src, size, want)
	}
}

func TestSkipLongLineCrossingChunks(t *testing.T) {
	src := []byte("1.2.3.4\n" + strings.Repeat("x", 205) + "\n8.8.8.8\n")
	for _, size := range []int{17, 20, 47, 64} {
		testChunkedIPs(t, src, size, []uint32{0x01020304, 0x08080808})
	}
}

type failingReader struct {
	read bool
}

var errInjected = errors.New("injected read failure")

func (r *failingReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, errInjected
	}
	r.read = true
	return copy(p, []byte("1.2.3.4\n")), nil
}

func TestReadErrorDrainsChunks(t *testing.T) {
	pool := &sync.Pool{New: func() any { return make([]byte, 32) }}
	chunks := make(chan ipChunk, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for chunk := range chunks {
			pool.Put(chunk.storage)
		}
	}()
	err := readChunksByLine(&failingReader{}, 32, chunks, pool)
	close(chunks)
	wg.Wait()
	if !errors.Is(err, errInjected) {
		t.Fatalf("got error %v, want %v", err, errInjected)
	}
}

func TestEndToEndInvalidAndNewlines(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "ips-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	data := "1.2.3.4\r\n256.1.1.1\nabc\n1.2.3.4\n8.8.8.8\n\n0.0.0.0\n1.2.3\n255.255.255.255"
	if _, err := io.WriteString(file, data); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		impl Counter
	}{{"atomic", &AtomicCounter{}}, {"bitset", &BitsetCounter{}}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.impl.CountUniqueIPs(file.Name())
			if err != nil || got != 4 {
				t.Fatalf("CountUniqueIPs = (%d, %v), want (4, nil)", got, err)
			}
		})
	}
}

func TestAtomicBigChunkBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("large integration test")
	}
	file, err := os.CreateTemp(t.TempDir(), "ips-boundary-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	// The 4 MiB chunk boundary falls inside an IPv4 address.
	line := []byte("1.2.3.4\n")
	for i := 0; i < atomicChunkSize/len(line)+20; i++ {
		if _, err := file.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := file.WriteString("255.255.255.255"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := (&AtomicCounter{}).CountUniqueIPs(file.Name())
	if err != nil || got != 2 {
		t.Fatalf("CountUniqueIPs = (%d, %v), want (2, nil)", got, err)
	}
}

// A very long malformed line must be skipped rather than counted or reported
// as a scanner capacity error. Both implementations must behave identically.
func TestSkipHugeInvalidLineBothCounters(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "ips-huge-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString("1.2.3.4\n" + strings.Repeat("x", 1<<20+100) + "\n8.8.8.8"); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, impl := range []struct {
		name string
		c    Counter
	}{
		{"atomic", &AtomicCounter{}}, {"bitset", &BitsetCounter{}},
	} {
		t.Run(impl.name, func(t *testing.T) {
			got, err := impl.c.CountUniqueIPs(file.Name())
			if err != nil || got != 2 {
				t.Fatalf("got (%d, %v), want (2, nil)", got, err)
			}
		})
	}
}
