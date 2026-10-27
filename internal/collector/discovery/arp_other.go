//go:build !darwin && !linux && !windows

package discovery

import (
	"errors"
	"net"
)

func readARP() (map[string]net.HardwareAddr, error) {
	return nil, errors.New("reading the ARP table is not supported on this OS")
}
