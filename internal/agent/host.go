package agent

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
)

// DefaultIDFile is where the agent keeps its stable ID, e.g.
// ~/Library/Application Support/netwatch/agent-id on macOS.
func DefaultIDFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "netwatch", "agent-id"), nil
}

// loadOrCreateID returns the ID stored at path, generating and saving one on
// first run. A stable ID means the hub sees the same machine across restarts
// even if its hostname or IP changes.
func loadOrCreateID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read agent id: %w", err)
	}

	id := strings.ToLower(rand.Text())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create agent id dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write agent id: %w", err)
	}
	return id, nil
}

func hostInfo(id, version string) *netwatchv1.RegisterRequest {
	hostname, _ := os.Hostname()
	return &netwatchv1.RegisterRequest{
		AgentId:      id,
		Hostname:     hostname,
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		AgentVersion: version,
		Ips:          localIPs(),
	}
}

// localIPs lists this machine's unicast addresses, skipping loopback and
// link-local (169.254.x.x / fe80::) ones, which are useless for display.
func localIPs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var ips []string
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() {
			continue
		}
		ips = append(ips, ipnet.IP.String())
	}
	return ips
}
