package counter

import "encoding/binary"

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
