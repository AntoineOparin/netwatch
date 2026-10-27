// Package procnet measures which apps on this computer use the network.
//
// Only macOS is supported: its `nettop` tool reports bytes per process
// without root. Windows needs ETW tracing (admin rights) and Linux needs eBPF
// or packet capture, so on those Supported() is false and the agent skips
// this collector.
package procnet

import (
	"cmp"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

const (
	MetricRx = "proc_rx_bytes_per_sec"
	MetricTx = "proc_tx_bytes_per_sec"

	// Interval is how often the agent samples apps. The hub uses it to turn
	// rates back into data volumes.
	Interval = 2 * time.Second

	// Apps below this combined rate are noise (background chatter).
	minBytesPerSec = 1024
	// At most this many apps per sample, busiest first.
	maxApps = 8
)

var ErrUnsupported = errors.New("per-app network usage isn't supported on this OS")

// entry is one process line from a nettop sample.
type entry struct {
	name string // truncated by nettop to ~15 characters
	pid  int
	in   uint64
	out  uint64
}

// cleanLine undoes what the pseudo-terminal adds to nettop's output: a
// carriage return on every line, and an echoed "^D" plus backspaces at the
// very start.
func cleanLine(line string) string {
	line = strings.TrimRight(line, "\r")
	if i := strings.LastIndexByte(line, '\b'); i >= 0 {
		line = line[i+1:]
	}
	return line
}

// parseNettopLine reads a CSV line like "curl.54588,4550119,520,". The
// process name and PID are joined with a dot; names can contain dots too,
// so the PID is whatever follows the last one.
func parseNettopLine(line string) (entry, bool) {
	f := strings.Split(line, ",")
	if len(f) < 3 {
		return entry{}, false
	}
	dot := strings.LastIndexByte(f[0], '.')
	if dot <= 0 {
		return entry{}, false
	}
	pid, err := strconv.Atoi(f[0][dot+1:])
	if err != nil {
		return entry{}, false
	}
	in, err1 := strconv.ParseUint(f[1], 10, 64)
	out, err2 := strconv.ParseUint(f[2], 10, 64)
	if err1 != nil || err2 != nil {
		return entry{}, false
	}
	return entry{name: f[0][:dot], pid: pid, in: in, out: out}, true
}

// appName turns an executable path into the name a person would recognize.
// Inside an app bundle that's the outermost .app, so Chrome's many helper
// processes all count as "Google Chrome":
//
//	/Applications/Google Chrome.app/Contents/.../Google Chrome Helper.app/... -> Google Chrome
//	/usr/bin/curl -> curl
func appName(exe string) string {
	if i := strings.Index(exe, ".app/"); i >= 0 {
		return filepath.Base(exe[:i])
	}
	return filepath.Base(exe)
}

// usage is one app's traffic over a sample interval, in bytes/sec.
type usage struct {
	app    string
	rx, tx float64
}

// summarize groups one sample's processes by app, converts bytes to rates,
// and keeps the busiest apps.
func summarize(entries []entry, interval time.Duration, name func(entry) string) []usage {
	byApp := map[string]*usage{}
	for _, e := range entries {
		if e.in == 0 && e.out == 0 {
			continue
		}
		app := name(e)
		u := byApp[app]
		if u == nil {
			u = &usage{app: app}
			byApp[app] = u
		}
		u.rx += float64(e.in) / interval.Seconds()
		u.tx += float64(e.out) / interval.Seconds()
	}
	var out []usage
	for _, u := range byApp {
		if u.rx+u.tx >= minBytesPerSec {
			out = append(out, *u)
		}
	}
	slices.SortFunc(out, func(a, b usage) int {
		if c := cmp.Compare(b.rx+b.tx, a.rx+a.tx); c != 0 {
			return c
		}
		return cmp.Compare(a.app, b.app)
	})
	return out[:min(len(out), maxApps)]
}

func samples(apps []usage, at time.Time) []collector.Sample {
	out := make([]collector.Sample, 0, 2*len(apps))
	for _, u := range apps {
		labels := map[string]string{"app": u.app}
		out = append(out,
			collector.Sample{Source: "procnet", Metric: MetricRx, Labels: labels, Value: u.rx, At: at},
			collector.Sample{Source: "procnet", Metric: MetricTx, Labels: labels, Value: u.tx, At: at},
		)
	}
	return out
}
