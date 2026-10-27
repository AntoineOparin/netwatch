package discovery

import (
	"net"
	"os"
)

func readARP() (map[string]net.HardwareAddr, error) {
	out, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return nil, err
	}
	return parseLinuxARP(string(out)), nil
}
