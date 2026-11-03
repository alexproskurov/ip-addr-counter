package counter

// parseIPv4Strict parses a dotted-decimal IPv4 address without allocations.
// It does not rely on net.ParseIP/netip or third-party parsers.
// A single trailing '\r' is accepted for CRLF input. Leading zeros are rejected.
func parseIPv4Strict(line []byte) (uint32, bool) {
	if n := len(line); n != 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	if len(line) < 7 || len(line) > 15 {
		return 0, false
	}

	var ip uint32
	pos := 0
	for part := 0; part < 4; part++ {
		if pos >= len(line) {
			return 0, false
		}
		begin := pos
		value := uint32(0)
		for pos < len(line) && line[pos] != '.' {
			c := line[pos]
			if c < '0' || c > '9' || pos-begin == 3 {
				return 0, false
			}
			value = value*10 + uint32(c-'0')
			if value > 255 {
				return 0, false
			}
			pos++
		}
		if pos == begin || (pos-begin > 1 && line[begin] == '0') {
			return 0, false
		}
		ip = ip<<8 | value
		if part == 3 {
			if pos == len(line) {
				return ip, true
			}
			return 0, false
		}
		if pos >= len(line) || line[pos] != '.' {
			return 0, false
		}
		pos++
	}
	return 0, false
}
