// Package series keeps recent samples in memory, grouped into time series,
// and fans new samples out to live subscribers (the dashboard's streams).
// Durable storage with rollups replaces the retention window in M3.
package series

import (
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

type Point struct {
	At    time.Time
	Value float64
}

// Series is every point for one (device, metric, labels) combination.
type Series struct {
	Device string
	Metric string
	Labels map[string]string
	Points []Point
}

type Store struct {
	retention time.Duration

	mu     sync.Mutex
	series map[string]*Series
	subs   map[int]chan collector.Sample
	nextID int
}

func NewStore(retention time.Duration) *Store {
	return &Store{
		retention: retention,
		series:    map[string]*Series{},
		subs:      map[int]chan collector.Sample{},
	}
}

// Append stores a sample, drops points older than the retention window, and
// passes the sample to subscribers. A subscriber that isn't keeping up misses
// samples rather than stalling every collector.
func (s *Store) Append(smp collector.Sample) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key(smp)
	ser, ok := s.series[k]
	if !ok {
		ser = &Series{Device: smp.Device, Metric: smp.Metric, Labels: maps.Clone(smp.Labels)}
		s.series[k] = ser
	}
	ser.Points = append(ser.Points, Point{At: smp.At, Value: smp.Value})
	cutoff := smp.At.Add(-s.retention)
	if i := firstAtOrAfter(ser.Points, cutoff); i > 0 {
		ser.Points = slices.Clone(ser.Points[i:])
	}

	for _, ch := range s.subs {
		select {
		case ch <- smp:
		default:
		}
	}
}

// Query returns copies of every series for metric, with points at or after
// since, sorted by series key so results are stable.
func (s *Store) Query(metric string, since time.Time) []Series {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := slices.Sorted(maps.Keys(s.series))
	var out []Series
	for _, k := range keys {
		ser := s.series[k]
		if ser.Metric != metric {
			continue
		}
		out = append(out, Series{
			Device: ser.Device,
			Metric: ser.Metric,
			Labels: maps.Clone(ser.Labels),
			Points: slices.Clone(ser.Points[firstAtOrAfter(ser.Points, since):]),
		})
	}
	return out
}

// Subscribe returns a channel of every new sample. Call the returned func to
// unsubscribe; the channel is not closed, so stop reading at that point.
func (s *Store) Subscribe(buffer int) (<-chan collector.Sample, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID
	s.nextID++
	ch := make(chan collector.Sample, buffer)
	s.subs[id] = ch
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.subs, id)
	}
}

// key identifies a series: device, metric and sorted labels.
func key(smp collector.Sample) string {
	var b strings.Builder
	b.WriteString(smp.Device)
	b.WriteByte('|')
	b.WriteString(smp.Metric)
	for _, k := range slices.Sorted(maps.Keys(smp.Labels)) {
		b.WriteByte('|')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(smp.Labels[k])
	}
	return b.String()
}

func firstAtOrAfter(points []Point, t time.Time) int {
	i, _ := slices.BinarySearchFunc(points, t, func(p Point, t time.Time) int {
		return p.At.Compare(t)
	})
	return i
}
