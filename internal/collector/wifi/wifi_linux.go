package wifi

import (
	"context"
	"os"
)

func read(context.Context) (reading, error) {
	out, err := os.ReadFile("/proc/net/wireless")
	if err != nil {
		return reading{}, errNotConnected
	}
	return parseLinux(string(out))
}
