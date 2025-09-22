# Implementations

## 1. Single-threaded

A simple implementation that reads the file line by line using `bufio.Scanner`.  
Each IPv4 address is parsed and stored in a bitset.  
At the end, the bitset count gives the number of unique IPs.

- Pros: simple, easy to read.
- Cons: slower for large files, uses one CPU core only.

## 2. Concurrent (worker pool + atomic bitmap)

A faster implementation that splits the input file into chunks,  
distributes them to workers (goroutines), and marks IPs in an atomic bitmap.

- Pros: much faster, scales with file size.
- Cons: higher memory usage.

Both implementations return the number of **unique IPv4 addresses** as a `uint64`.
