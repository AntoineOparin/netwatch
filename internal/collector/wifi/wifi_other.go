//go:build !darwin && !windows && !linux

package wifi

import "context"

func read(context.Context) (reading, error) { return reading{}, errNotConnected }
