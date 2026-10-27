package collector

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// flaky emits one sample per run and fails its first run.
type flaky struct{ runs int }

func (f *flaky) Name() string { return "flaky" }

func (f *flaky) Run(ctx context.Context, out chan<- Sample) error {
	f.runs++
	out <- Sample{Metric: "m", Value: float64(f.runs)}
	if f.runs == 1 {
		return errors.New("boom")
	}
	<-ctx.Done()
	return nil
}

func TestRunAllRestartsFailedCollector(t *testing.T) {
	restartDelay = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())

	got := make(chan float64, 2)
	done := make(chan struct{})
	go func() {
		RunAll(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), func(s Sample) { got <- s.Value }, &flaky{})
		close(done)
	}()

	for want := 1.0; want <= 2; want++ {
		select {
		case v := <-got:
			if v != want {
				t.Fatalf("sample %v, want %v", v, want)
			}
		case <-time.After(time.Second):
			t.Fatal("collector was not restarted")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunAll did not return after cancel")
	}
}
