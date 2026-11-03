# IP Address Counter

A Go project for counting **unique, valid IPv4 addresses** in large, newline-delimited text files. It compares a concurrent atomic bitmap with a single-threaded adaptive bitmap, without third-party IP parsing or bitset libraries.

## Usage

```bash
go run . atomic path/to/ips.txt
go run . bitset path/to/ips.txt
```

Both commands print `Unique IPs: N`.

Optional CPU profiling (disabled by default):

```bash
go run . -cpuprofile cpu.prof atomic path/to/ips.txt
```

Each line must contain one dotted-decimal IPv4 address. CRLF and an unterminated final line are supported. Invalid, empty, or overlong lines are ignored. The parser rejects out-of-range octets, extra components, non-digits, whitespace, and leading zeros in multi-digit octets.

## Implementations

### Concurrent atomic bitmap (`atomic`)

- Reads the file in 4 MiB blocks, preserving addresses split across block boundaries.
- Reuses owned buffers through a bounded worker pool and `sync.Pool`.
- Parses IPv4 with a custom, nonallocating parser.
- Sets bits in a shared 512 MiB bitmap using compare-and-swap (CAS).
- Counts successful first-time bit transitions per worker, then sums worker-local counts. No final full-bitmap scan is needed.

The main tradeoff is the fixed bitmap allocation (512 MiB), even for a small number of distinct addresses.

### Single-threaded adaptive bitmap (`bitset`)

- Starts with a sparse collection of 8 KiB pages, each covering one IPv4 `/16` range.
- Allocates pages on demand and counts a new address as soon as its bit is set.
- After more than 64 distinct pages are needed, switches to a dense 512 MiB bitmap, preserving all previously recorded bits.
- Uses a custom bitmap rather than `github.com/bits-and-blooms/bitset`.

This saves substantial memory for clustered addresses. Highly scattered addresses may trigger the dense representation.

## Correctness and tests

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
```

Regression tests cover IPv4 parsing and malformed input, Windows CRLF, missing terminal newline, long lines, chunk boundaries, duplicate counting under contention, and adaptive-bitmap switching. A parser fuzz target is also included.

## Benchmarks

```bash
go test -run '^$' -bench '^BenchmarkIPDistributions' -benchtime=10x -count=5 -benchmem ./counter
```

The following measurements are **medians of five runs on Linux, AMD Ryzen 5 2600**, using the same `BenchmarkIPDistributions` cases for upstream `main` (`be2802d`) and the optimized version. They are illustrative, not guaranteed on other hardware.

| Data distribution | Implementation | Upstream | Optimized | Speedup |
| --- | --- | ---: | ---: | ---: |
| 16 repeated addresses | atomic | 227.5 ms | 109.6 ms | 2.08× |
| Sequential unique addresses | atomic | 246.3 ms | 106.3 ms | 2.32× |
| Scattered unique addresses | atomic | 550.5 ms | 131.8 ms | 4.18× |
| 16 repeated addresses | bitset | 184.1 ms | 126.0 ms | 1.46× |
| Sequential unique addresses | bitset | 218.8 ms | 140.7 ms | 1.56× |
| Scattered unique addresses | bitset | 504.2 ms | 465.1 ms | 1.08× |

The Go benchmark `B/op` figure reports **total bytes allocated per operation**, not peak resident memory. For the repeated and sequential cases, adaptive-bitset allocations were approximately 598 KiB and 967 KiB, respectively, versus about 537 MB for the original dense bitset.

## Why this project?

To explore efficient IPv4 parsing, memory representation, concurrent file processing, atomic operations, and reproducible performance testing in Go.
