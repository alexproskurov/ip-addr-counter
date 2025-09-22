package counter

import (
	"bufio"
	"encoding/binary"
	"math"
	"os"

	"github.com/bits-and-blooms/bitset"
)

type BitsetCounter struct{}

var _ Counter = (*BitsetCounter)(nil)

func (c *BitsetCounter) CountUniqueIPs(filePath string) (uint64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	bs := bitset.New(math.MaxUint32)
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		ip := parseIPv4Bytes(scanner.Bytes())
		bs.Set(uint(ip))
	}
	return uint64(bs.Count()), nil
}

func parseIPv4Bytes(input []byte) uint32 {
	var ip [4]byte
	var ipOffset int
	for _, c := range input {
		if c == '.' {
			ipOffset++
			continue
		}
		ip[ipOffset] = ip[ipOffset]*10 + (c - '0')
	}
	return binary.BigEndian.Uint32(ip[:])
}
