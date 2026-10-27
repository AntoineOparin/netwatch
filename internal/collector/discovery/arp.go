package discovery

import (
	"bufio"
	"net"
	"strings"
)

// The ARP table maps IP addresses to MAC (hardware) addresses for devices on
// the local network. The OS fills it whenever it needs to send something to
// a local IP: it broadcasts "who has 192.168.0.42?" and the owner answers
// with its MAC. Devices that ignore pings still answer ARP, or they couldn't
// receive anything at all.

// parseDarwinARP reads `arp -an` output:
//
//	? (192.168.0.1) at 22:6a:94:0:1:2 on en0 ifscope [ethernet]
//	? (192.168.0.9) at (incomplete) on en0 ifscope [ethernet]
func parseDarwinARP(out string) map[string]net.HardwareAddr {
	table := map[string]net.HardwareAddr{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 || f[2] != "at" {
			continue
		}
		ip := strings.Trim(f[1], "()")
		if mac := parseMAC(f[3]); mac != nil {
			table[ip] = mac
		}
	}
	return table
}

// parseLinuxARP reads /proc/net/arp:
//
//	IP address       HW type     Flags       HW address            Mask     Device
//	192.168.0.1      0x1         0x2         22:6a:94:00:01:02     *        eth0
//
// Flags 0x2 means the entry is complete (the device answered).
func parseLinuxARP(out string) map[string]net.HardwareAddr {
	table := map[string]net.HardwareAddr{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 || f[2] != "0x2" {
			continue
		}
		if mac := parseMAC(f[3]); mac != nil {
			table[f[0]] = mac
		}
	}
	return table
}

// parseWindowsARP reads `arp -a` output:
//
//	192.168.0.1           22-6a-94-00-01-02     dynamic
func parseWindowsARP(out string) map[string]net.HardwareAddr {
	table := map[string]net.HardwareAddr{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 3 || f[2] != "dynamic" || net.ParseIP(f[0]) == nil {
			continue
		}
		if mac := parseMAC(strings.ReplaceAll(f[1], "-", ":")); mac != nil {
			table[f[0]] = mac
		}
	}
	return table
}

// parseMAC accepts macOS's unpadded form ("6:c7:ef:1:2:3") as well as the
// usual one, and rejects broadcast and multicast addresses.
func parseMAC(s string) net.HardwareAddr {
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return nil
	}
	for i, p := range parts {
		if len(p) == 1 {
			parts[i] = "0" + p
		}
	}
	mac, err := net.ParseMAC(strings.Join(parts, ":"))
	if err != nil || mac[0]&1 == 1 { // low bit of the first byte: multicast/broadcast
		return nil
	}
	return mac
}

// isPrivateMAC reports whether a MAC is locally administered (second-lowest
// bit of the first byte). Phones, tablets and laptops make one up per Wi-Fi
// network for privacy, so it says nothing about the maker.
func isPrivateMAC(mac net.HardwareAddr) bool {
	return len(mac) > 0 && mac[0]&2 != 0
}
