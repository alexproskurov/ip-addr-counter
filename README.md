# IP Address Counter

A Go project for counting **unique IPv4 addresses** in very large text files.

This repository demonstrates two approaches:

* a simple **single-threaded** implementation,
* and a **concurrent worker-pool** implementation using an atomic bitmap.

It’s a practical example of working with bitmaps, atomic operations, and efficient file reading in Go.

---

## Implementations

### 1. Single-threaded

Reads the file line by line using `bufio.Scanner`. Each IPv4 address is parsed and stored in a bitset. At the end, the bitset count gives the number of unique IPs.

* **Pros:** simple, easy to read.
* **Cons:** slower for large files, only uses one CPU core.

### 2. Concurrent (worker pool + atomic bitmap)

Reads the file in large chunks (64 MB), splits them by line, and distributes the chunks to worker goroutines. Each worker sets bits in a shared atomic bitmap (`[]uint32`).

* **Pros:** faster on large files, utilizes multiple CPU cores.
* **Cons:** higher memory usage, more complex code.

---

## Benchmarks

Run benchmarks:

```bash
go test -bench=. -benchmem ./counter
```

Example results (with \~140M IPv4 addresses, 14 unique):

```bash
BenchmarkCounters/AtomicCounter-8                 1 1085033875 ns/op 2614236184 B/op       85 allocs/op
BenchmarkCounters/BitsetCounter-8                 1 4676660167 ns/op 536936680 B/op        7 allocs/op
```

* **AtomicCounter (concurrent):** lower latency due to parallelism, but higher memory usage.
* **BitsetCounter (single-threaded):** simpler and leaner, but slower on large files.

---

## Output

Both implementations return the number of unique IPv4 addresses as a `uint64`.

---

## Why this project?

This project was built to experiment with:

* parsing IPv4 addresses efficiently,
* working with bitmaps and atomic operations,
* comparing single-threaded vs concurrent approaches in Go,
* benchmarking real-world file processing performance.

---
