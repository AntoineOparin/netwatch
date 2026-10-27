package series

import (
	"testing"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

func TestStoreRetentionAndQuery(t *testing.T) {
	s := NewStore(10 * time.Second)
	t0 := time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)
	gw := map[string]string{"target": "gateway"}
	inet := map[string]string{"target": "internet"}

	for i := range 15 {
		at := t0.Add(time.Duration(i) * time.Second)
		s.Append(collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: float64(i), At: at})
		s.Append(collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: inet, Value: float64(i), At: at})
	}
	s.Append(collector.Sample{Device: "hub", Metric: "other", Value: 1, At: t0})

	got := s.Query("rtt_ms", time.Time{})
	if len(got) != 2 {
		t.Fatalf("got %d series, want 2", len(got))
	}
	// Last point at t0+14s, retention 10s: points from t0+4s on survive.
	if pts := got[0].Points; len(pts) != 11 || pts[0].Value != 4 || pts[10].Value != 14 {
		t.Errorf("gateway points = %v, want values 4..14", pts)
	}

	recent := s.Query("rtt_ms", t0.Add(12*time.Second))
	if n := len(recent[1].Points); n != 3 {
		t.Errorf("since t0+12s: got %d points, want 3", n)
	}
}

func TestStoreSubscribe(t *testing.T) {
	s := NewStore(time.Minute)
	ch, unsubscribe := s.Subscribe(1)

	s.Append(collector.Sample{Metric: "a", At: time.Now()})
	s.Append(collector.Sample{Metric: "b", At: time.Now()}) // buffer full: dropped, not blocking

	if got := <-ch; got.Metric != "a" {
		t.Fatalf("got %q, want a", got.Metric)
	}
	unsubscribe()
	s.Append(collector.Sample{Metric: "c", At: time.Now()})
	select {
	case got := <-ch:
		t.Fatalf("received %q after unsubscribe", got.Metric)
	default:
	}
}
