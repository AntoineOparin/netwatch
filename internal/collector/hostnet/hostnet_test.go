package hostnet

import (
	"context"
	"testing"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

func TestRates(t *testing.T) {
	t0 := time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)
	readings := []counters{
		{rx: 1_000, tx: 500},
		{rx: 3_000, tx: 1_500}, // +2000 rx, +1000 tx over 2 s
		{rx: 100, tx: 100},     // reset: skipped
		{rx: 1_100, tx: 600},   // +1000, +500 over 2 s
	}
	step := 0
	c := &Collector{
		interval: time.Millisecond,
		uplink:   func() (string, error) { return "en0", nil },
		read: func(iface string) (counters, error) {
			r := readings[min(step, len(readings)-1)]
			step++
			return r, nil
		},
		now: func() time.Time { return t0.Add(time.Duration(step) * 2 * time.Second) },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan collector.Sample)
	go c.Run(ctx, out)

	want := []struct {
		metric string
		value  float64
	}{
		{MetricRx, 1000}, {MetricTx, 500},
		{MetricRx, 500}, {MetricTx, 250},
	}
	for _, w := range want {
		got := <-out
		if got.Metric != w.metric || got.Value != w.value || got.Labels["iface"] != "en0" {
			t.Fatalf("got %s=%v on %q, want %s=%v on en0", got.Metric, got.Value, got.Labels["iface"], w.metric, w.value)
		}
	}
}
