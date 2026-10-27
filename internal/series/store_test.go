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

func TestCountPoints(t *testing.T) {
	t0 := time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)
	now := t0.Add(time.Minute)
	s := openTest(t, &now)
	internet := map[string]string{"target": "internet"}
	for i := range 10 {
		v := 20.0
		if i%3 == 0 {
			v = math.NaN() // lost pings still count as "watched"
		}
		mustAppend(t, s, collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: internet, Value: v, At: t0.Add(time.Duration(i) * time.Second)})
		mustAppend(t, s, collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: map[string]string{"target": "gateway"}, Value: 4, At: t0.Add(time.Duration(i) * time.Second)})
	}
	n, err := s.CountPoints("rtt_ms", t0.Add(2*time.Second), func(_ string, l map[string]string) bool { return l["target"] == "internet" })
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 {
		t.Fatalf("got %d, want 8 internet pings from t0+2s", n)
	}
}

func TestLast24HoursReadsRawSamples(t *testing.T) {
	now := time.Date(2026, 10, 27, 21, 0, 30, 0, time.UTC)
	s := openTest(t, &now)
	mustAppend(t, s, collector.Sample{Device: "hub", Metric: "m", Value: 1, At: now.Add(-10 * time.Second)})

	// No rollups exist yet; a 24 h window computed a moment ago must still
	// see the raw sample.
	since := now.Add(-RawRetention - time.Millisecond)
	if n, err := s.CountPoints("m", since, func(string, map[string]string) bool { return true }); err != nil || n != 1 {
		t.Fatalf("CountPoints = %d, %v; want 1", n, err)
	}
	if got, err := s.Query("m", since); err != nil || len(got) != 1 {
		t.Fatalf("Query = %v, %v; want the raw sample", got, err)
	}
}

func TestHourRetentionAndSeriesPruning(t *testing.T) {
	t0 := time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)
	now := t0
	s := openTest(t, &now)
	app := map[string]string{"app": "OneDrive"}
	mustAppend(t, s, collector.Sample{Device: "mac", Metric: "m", Labels: app, Value: 1, At: t0})

	// Roll it up into minutes and hours.
	now = t0.Add(2 * time.Hour)
	if err := s.maintain(); err != nil {
		t.Fatal(err)
	}
	var hours int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM samples_1h`).Scan(&hours); err != nil || hours != 1 {
		t.Fatalf("hourly rows = %d, %v; want 1", hours, err)
	}

	// 400 days later, past the 1-year default, everything about it is gone,
	// including its row in the series table.
	now = t0.Add(400 * 24 * time.Hour)
	if err := s.maintain(); err != nil {
		t.Fatal(err)
	}
	var rows, series int
	if err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM samples_1h), (SELECT COUNT(*) FROM series)`).Scan(&rows, &series); err != nil {
		t.Fatal(err)
	}
	if rows != 0 || series != 0 {
		t.Fatalf("after a year: %d hourly rows, %d series; want none", rows, series)
	}

	// The same series can be written again: the ID cache didn't keep the
	// deleted ID.
	mustAppend(t, s, collector.Sample{Device: "mac", Metric: "m", Labels: app, Value: 2, At: now})
	got, err := s.Query("m", now.Add(-time.Minute))
	if err != nil || len(got) != 1 || got[0].Points[0].Value != 2 {
		t.Fatalf("Query after pruning = %v, %v; want the new point", got, err)
	}
}

func TestSetHourRetentionMustOutliveMinutes(t *testing.T) {
	now := time.Now()
	s := openTest(t, &now)
	if err := s.SetHourRetention(7 * 24 * time.Hour); err == nil {
		t.Error("a week is shorter than the 30 days of minutes; want an error")
	}
	if err := s.SetHourRetention(90 * 24 * time.Hour); err != nil {
		t.Error(err)
	}
}
