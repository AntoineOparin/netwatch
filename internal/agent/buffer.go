package agent

import (
	"sync"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

// buffer holds samples until they are reported. When full it drops the
// oldest: recent data is what the dashboard needs most.
type buffer struct {
	mu      sync.Mutex
	max     int
	samples []collector.Sample
}

func (b *buffer) add(s collector.Sample) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.samples = append(b.samples, s)
	b.trim()
}

// take empties the buffer and returns what it held.
func (b *buffer) take() []collector.Sample {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.samples
	b.samples = nil
	return out
}

// putBack returns samples from a failed report, ahead of anything that
// arrived since, so order is kept.
func (b *buffer) putBack(samples []collector.Sample) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.samples = append(samples, b.samples...)
	b.trim()
}

func (b *buffer) len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.samples)
}

// trim drops the oldest samples past max. Callers hold b.mu.
func (b *buffer) trim() {
	if over := len(b.samples) - b.max; over > 0 {
		b.samples = append([]collector.Sample(nil), b.samples[over:]...)
	}
}
