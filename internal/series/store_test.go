package series

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

func openTest(t *testing.T, now *time.Time) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s.now = func() time.Time { return *now }
	return s
}

func mustAppend(t *testing.T, s *Store, smp collector.Sample) {
	t.Helper()
	if err := s.Append(smp); err != nil {
		t.Fatal(err)
	}
}

func TestStoreQueryRaw(t *testing.T) {
	t0 := time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)
	now := t0.Add(time.Minute)
	s := openTest(t, &now)
	gw := map[string]string{"target": "gateway"}
	inet := map[string]string{"target": "internet"}

	for i := range 15 {
		at := t0.Add(time.Duration(i) * time.Second)
		mustAppend(t, s, collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: float64(i), At: at})
		mustAppend(t, s, collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: inet, Value: math.NaN(), At: at})
	}
	mustAppend(t, s, collector.Sample{Device: "hub", Metric: "other", Value: 1, At: t0})

	got, err := s.Query("rtt_ms", t0.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d series, want 2", len(got))
	}
	if got[0].Labels["target"] != "gateway" || len(got[0].Points) != 5 || got[0].Points[0].Value != 10 {
		t.Errorf("gateway = %+v, want 5 points starting at 10", got[0])
	}
	if !math.IsNaN(got[1].Points[0].Value) {
		t.Errorf("NaN should survive the round trip as NaN, got %v", got[1].Points[0].Value)
	}
}

func TestStoreRollupAndRetention(t *testing.T) {
	t0 := time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)
	now := t0
	s := openTest(t, &now)

	// Minute 21:00 gets 10, 20, lost; minute 21:01 gets 40.
	for i, v := range []float64{10, 20, math.NaN()} {
		mustAppend(t, s, collector.Sample{Device: "d", Metric: "m", Value: v, At: t0.Add(time.Duration(i) * time.Second)})
	}
	mustAppend(t, s, collector.Sample{Device: "d", Metric: "m", Value: 40, At: t0.Add(time.Minute)})

	now = t0.Add(time.Minute + 30*time.Second) // 21:01 hasn't finished yet
	if err := s.maintain(); err != nil {
		t.Fatal(err)
	}
	var n, missing int
	var sum float64
	if err := s.db.QueryRow(`SELECT n, n_missing, sum FROM samples_1m`).Scan(&n, &missing, &sum); err != nil {
		t.Fatal(err)
	}
	if n != 3 || missing != 1 || sum != 30 {
		t.Errorf("21:00 rollup = n %d, missing %d, sum %v; want 3, 1, 30", n, missing, sum)
	}

	// Two days later the raw rows are gone, and a long query reads the
	// per-minute averages instead.
	now = t0.Add(48 * time.Hour)
	if err := s.maintain(); err != nil {
		t.Fatal(err)
	}
	var raw int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM samples`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != 0 {
		t.Errorf("%d raw samples survived retention", raw)
	}
	got, err := s.Query("m", t0.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Points) != 2 || got[0].Points[0].Value != 15 || got[0].Points[1].Value != 40 {
		t.Fatalf("minute averages = %+v, want 15 then 40", got)
	}
}

func TestStoreSubscribe(t *testing.T) {
	now := time.Now()
	s := openTest(t, &now)
	ch, unsubscribe := s.Subscribe(1)

	mustAppend(t, s, collector.Sample{Metric: "a", At: now})
	mustAppend(t, s, collector.Sample{Metric: "b", At: now}) // buffer full: dropped, not blocking

	if got := <-ch; got.Metric != "a" {
		t.Fatalf("got %q, want a", got.Metric)
	}
	unsubscribe()
	mustAppend(t, s, collector.Sample{Metric: "c", At: now})
	select {
	case got := <-ch:
		t.Fatalf("received %q after unsubscribe", got.Metric)
	default:
	}
}
