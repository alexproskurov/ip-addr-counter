package counter_test

import (
	"os"
	"testing"

	"github.com/alexproskurov/ip-addr-counter/counter"
)

func writeTempFile(t *testing.T, addresses []string, repeatTimes int) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "ips_test_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()

	for i := 0; i < repeatTimes; i++ {
		for _, addr := range addresses {
			_, err := tmpFile.WriteString(addr + "\n")
			if err != nil {
				t.Fatalf("failed writing IP address %q: %v", addr, err)
			}
		}
	}

	return tmpFile.Name()
}

var tests = []struct {
	name        string
	addresses   []string
	expected    uint64
	repeatTimes int
}{
	{
		name: "unique IPs",
		addresses: []string{
			"5.212.38.46", "79.174.235.110", "7.18.194.41",
			"52.215.165.104", "15.161.241.93", "127.233.43.195",
			"242.55.106.246", "230.42.235.27", "85.244.97.117",
			"206.223.44.110", "104.122.33.7", "58.161.248.121",
			"204.183.223.247", "151.225.183.115"},
		expected:    14,
		repeatTimes: 100000,
	},
	{
		name:        "duplicate IPs",
		addresses:   []string{"192.168.0.1", "192.168.0.1", "10.0.0.1"},
		expected:    2,
		repeatTimes: 1000,
	},
	{
		name:        "empty file",
		addresses:   []string{},
		expected:    0,
		repeatTimes: 1,
	},
}

var counterImpls = []struct {
	name string
	c    counter.Counter
}{
	{"AtomicCounter", &counter.AtomicCounter{}},
	{"BitsetCounter", &counter.BitsetCounter{}},
}

func TestIPCounters(t *testing.T) {
	for _, impl := range counterImpls {
		for _, tt := range tests {
			t.Run(impl.name+"_"+tt.name, func(t *testing.T) {
				fileName := writeTempFile(t, tt.addresses, tt.repeatTimes)
				defer os.Remove(fileName)

				count, err := impl.c.CountUniqueIPs(fileName)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if count != tt.expected {
					t.Errorf("%s/%s: got %d, want %d", impl.name, tt.name, count, tt.expected)
				}
			})
		}
	}
}
