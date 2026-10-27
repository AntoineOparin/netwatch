package discovery

import (
	"net"
	"os/exec"
)

func readARP() (map[string]net.HardwareAddr, error) {
	out, err := exec.Command("arp", "-a").Output()
	if err != nil {
		return nil, err
	}
	return parseWindowsARP(string(out)), nil
}
