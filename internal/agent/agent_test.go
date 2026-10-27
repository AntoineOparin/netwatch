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

	"github.com/AntoineOparin/netwatch/internal/hub"
	"github.com/AntoineOparin/netwatch/internal/series"
)

// TestReRegisterAfterHubRestart runs a real hub in-process, then swaps it for
// a fresh one (an empty registry, like after a restart) and checks the agent
// finds its way back.
func TestReRegisterAfterHubRestart(t *testing.T) {
	var current atomic.Pointer[hub.Registry]
	newHub := func() http.Handler {
		reg := hub.NewRegistry()
		current.Store(reg)
		return hub.NewServer(reg, series.NewStore(time.Minute), 20*time.Millisecond).Handler(http.NotFoundHandler())
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
			HubURL:  srv.URL,
			IDFile:  filepath.Join(t.TempDir(), "agent-id"),
			Version: "test",
			Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()

	waitForAgent(t, &current)

	restarted := newHub() // the new registry has never heard of the agent
	handler.Store(&restarted)
	waitForAgent(t, &current)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run returned %v after cancel, want nil", err)
	}
}

func waitForAgent(t *testing.T, reg *atomic.Pointer[hub.Registry]) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if agents := reg.Load().List(); len(agents) == 1 {
			if agents[0].AgentVersion != "test" {
				t.Fatalf("registered version = %q, want test", agents[0].AgentVersion)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("agent did not register in time")
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
