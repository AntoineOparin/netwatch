package discovery

import (
	"net"
	"os/exec"
)

func readARP() (map[string]net.HardwareAddr, error) {
	out, err := exec.Command("arp", "-an").Output()
	if err != nil {
		return nil, err
	}
	return parseDarwinARP(string(out)), nil
}
