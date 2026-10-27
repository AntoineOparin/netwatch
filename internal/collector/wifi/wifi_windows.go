package wifi

import (
	"context"
	"os/exec"
)

func read(ctx context.Context) (reading, error) {
	out, err := exec.CommandContext(ctx, "netsh", "wlan", "show", "interfaces").Output()
	if err != nil {
		// No WLAN service (a desktop without Wi-Fi) also lands here.
		return reading{}, errNotConnected
	}
	return parseWindows(string(out))
}
