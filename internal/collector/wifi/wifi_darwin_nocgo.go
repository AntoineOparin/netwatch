//go:build darwin && !cgo

package wifi

import "context"

// Without cgo there's no way to reach CoreWLAN. `system_profiler` would
// work but triggers a Wi-Fi scan that causes 100+ ms lag spikes, so a
// build without cgo reports no Wi-Fi rather than disturbing the network.
func read(context.Context) (reading, error) { return reading{}, errNotConnected }
