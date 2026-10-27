package agent

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/devices"
	"github.com/AntoineOparin/netwatch/internal/hub"
	"github.com/AntoineOparin/netwatch/internal/incident"
	"github.com/AntoineOparin/netwatch/internal/series"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// counter emits an increasing value every few milliseconds.
type counter struct{}

func (counter) Name() string { return "counter" }

func (counter) Run(ctx context.Context, out chan<- collector.Sample) error {
	for i := 0; ; i++ {
		select {
		case out <- collector.Sample{Metric: "count", Value: float64(i), At: time.Now()}:
		case <-ctx.Done():
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestReportsSurviveHubRestart runs a real hub in-process, then swaps it for
// a fresh one (an empty registry, like after a restart) and checks the agent
// registers again and keeps delivering samples.
func TestReportsSurviveHubRestart(t *testing.T) {
	store, err := series.Open(filepath.Join(t.TempDir(), "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	incidents, err := incident.OpenStore(store.DB())
	if err != nil {
		t.Fatal(err)
	}
	devs, err := devices.OpenStore(store.DB())
	if err != nil {
		t.Fatal(err)
	}

	var registry atomic.Pointer[hub.Registry]
	newHub := func() http.Handler {
		reg := hub.NewRegistry()
		registry.Store(reg)
		return hub.NewServer(hub.Config{
			Registry:       reg,
			Store:          store,
			Incidents:      incidents,
			Devices:        devs,
			ReportInterval: 20 * time.Millisecond,
			Log:            quiet,
		}).Handler(http.NotFoundHandler())
	}
	var handler atomic.Pointer[http.Handler]
	h := newHub()
	handler.Store(&h)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		(*handler.Load()).ServeHTTP(w, r)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Config{
			HubURL:     srv.URL,
			IDFile:     filepath.Join(t.TempDir(), "agent-id"),
			Version:    "test",
			Logger:     quiet,
			Collectors: []collector.Collector{counter{}},
		})
	}()

	waitFor(t, "first registration", func() bool { return len(registry.Load().List()) == 1 })
	waitFor(t, "first samples", func() bool { return countStored(t, store) > 0 })

	restarted := newHub() // the new registry has never heard of the agent
	handler.Store(&restarted)
	waitFor(t, "registration after restart", func() bool { return len(registry.Load().List()) == 1 })

	before := countStored(t, store)
	waitFor(t, "samples after restart", func() bool { return countStored(t, store) > before })

	got, err := store.Query("count", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Every sample the collector produced up to the newest stored one made
	// it, including those taken while the agent was re-registering.
	points := got[0].Points
	if last := points[len(points)-1].Value; int(last)+1 != len(points) {
		t.Errorf("stored %d samples but the newest is #%v: some were lost", len(points), last)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run returned %v after cancel, want nil", err)
	}
}

func countStored(t *testing.T, store *series.Store) int {
	t.Helper()
	got, err := store.Query("count", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		return 0
	}
	return len(got[0].Points)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestLoadOrCreateIDIsStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "agent-id")
	first, err := loadOrCreateID(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreateID(path)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("ids = %q, %q, want the same non-empty id", first, second)
	}
}

func TestBufferDropsOldest(t *testing.T) {
	b := &buffer{max: 3}
	for i := range 4 {
		b.add(collector.Sample{Value: float64(i)})
	}
	taken := b.take() // 1, 2, 3
	b.add(collector.Sample{Value: 4})
	b.putBack(taken) // 1, 2, 3, 4 -> over max, 1 dropped
	got := b.take()
	if len(got) != 3 || got[0].Value != 2 || got[2].Value != 4 {
		t.Fatalf("got %v, want values 2, 3, 4", got)
	}
}
