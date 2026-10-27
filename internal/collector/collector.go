// Package collector is the seam every data source plugs into. Probes, agent
// host stats, LAN discovery and (later) packet capture all emit Samples, so
// storage, analysis and the UI never care where a number came from.
package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Sample is one measurement.
type Sample struct {
	Source string            // collector that produced it, e.g. "probe"
	Device string            // machine it describes: "hub", an agent ID, later a MAC
	Metric string            // e.g. "rtt_ms"
	Labels map[string]string // extra dimensions, e.g. target=gateway
	Value  float64
	At     time.Time
}

type Collector interface {
	Name() string
	// Run emits samples until ctx is cancelled. Returning an error before
	// that means the collector broke and should be restarted.
	Run(ctx context.Context, out chan<- Sample) error
}

// restartDelay is how long a failed collector waits before running again.
var restartDelay = 5 * time.Second

// RunAll runs every collector and hands each sample to sink, until ctx is
// cancelled. A failing collector is logged and restarted: one broken source
// (say, no permission for ICMP) shouldn't take the others down.
func RunAll(ctx context.Context, log *slog.Logger, sink func(Sample), collectors ...Collector) {
	out := make(chan Sample, 256)
	var wg sync.WaitGroup
	for _, c := range collectors {
		wg.Go(func() {
			for {
				err := c.Run(ctx, out)
				if ctx.Err() != nil {
					return
				}
				log.Warn("collector stopped, restarting", "collector", c.Name(), "err", err, "retry_in", restartDelay)
				select {
				case <-ctx.Done():
					return
				case <-time.After(restartDelay):
				}
			}
		})
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	for s := range out {
		sink(s)
	}
}
