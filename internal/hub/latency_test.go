package hub

import (
	"context"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/gen/netwatch/v1/netwatchv1connect"
	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/collector/hostnet"
	"github.com/AntoineOparin/netwatch/internal/series"
)

type testHub struct {
	store     *series.Store
	registry  *Registry
	dashboard netwatchv1connect.DashboardServiceClient
	agents    netwatchv1connect.AgentServiceClient
}

func newTestHub(t *testing.T) *testHub {
	t.Helper()
	store, err := series.Open(filepath.Join(t.TempDir(), "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	reg := NewRegistry()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(NewServer(reg, store, time.Second, quiet).Handler(http.NotFoundHandler()))
	t.Cleanup(srv.Close)
	return &testHub{
		store:     store,
		registry:  reg,
		dashboard: netwatchv1connect.NewDashboardServiceClient(srv.Client(), srv.URL),
		agents:    netwatchv1connect.NewAgentServiceClient(srv.Client(), srv.URL),
	}
}

func (h *testHub) append(t *testing.T, smp collector.Sample) {
	t.Helper()
	if err := h.store.Append(smp); err != nil {
		t.Fatal(err)
	}
}

func TestWatchLatencySendsHistoryThenLive(t *testing.T) {
	h := newTestHub(t)
	gw := map[string]string{"target": "gateway", "addr": "192.168.0.1"}
	now := time.Now()
	h.append(t, collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: 4.5, At: now.Add(-2 * time.Second)})
	h.append(t, collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: math.NaN(), At: now.Add(-time.Second)})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := h.dashboard.WatchLatency(ctx, connect.NewRequest(&netwatchv1.WatchLatencyRequest{
		History: durationpb.New(time.Minute),
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	if !stream.Receive() {
		t.Fatalf("no history message: %v", stream.Err())
	}
	history := stream.Msg().GetPoints()
	if len(history) != 2 {
		t.Fatalf("history has %d points, want 2", len(history))
	}
	if history[0].GetRttMs() != 4.5 || history[0].GetTarget() != netwatchv1.LatencyTarget_LATENCY_TARGET_GATEWAY {
		t.Errorf("first point = %v", history[0])
	}
	if !history[1].GetLost() {
		t.Errorf("NaN sample should arrive as lost: %v", history[1])
	}

	// A duplicate of the last history point must not be re-sent; a newer one must.
	h.append(t, collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: math.NaN(), At: now.Add(-time.Second)})
	h.append(t, collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: 6, At: now})
	if !stream.Receive() {
		t.Fatalf("no live message: %v", stream.Err())
	}
	if live := stream.Msg().GetPoints(); len(live) != 1 || live[0].GetRttMs() != 6 {
		t.Fatalf("live points = %v, want one point of 6 ms", live)
	}
}

// An agent whose clock runs an hour slow reports a sample; it must land at
// the hub's "now", not an hour ago, and stream out as throughput.
func TestReportCorrectsClockSkew(t *testing.T) {
	h := newTestHub(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.agents.Report(ctx, connect.NewRequest(&netwatchv1.ReportRequest{AgentId: "dads-laptop"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("report before register: err = %v, want NOT_FOUND", err)
	}
	if _, err := h.agents.Register(ctx, connect.NewRequest(&netwatchv1.RegisterRequest{AgentId: "dads-laptop"})); err != nil {
		t.Fatal(err)
	}

	agentNow := time.Now().Add(-time.Hour)
	_, err = h.agents.Report(ctx, connect.NewRequest(&netwatchv1.ReportRequest{
		AgentId: "dads-laptop",
		SentAt:  timestamppb.New(agentNow),
		Samples: []*netwatchv1.Sample{{
			Metric: hostnet.MetricTx,
			Labels: map[string]string{"iface": "en0"},
			At:     timestamppb.New(agentNow.Add(-time.Second)),
			Value:  1_250_000,
		}},
	}))
	if err != nil {
		t.Fatal(err)
	}

	stream, err := h.dashboard.WatchThroughput(ctx, connect.NewRequest(&netwatchv1.WatchThroughputRequest{
		History: durationpb.New(time.Minute),
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if !stream.Receive() {
		t.Fatalf("no history: %v", stream.Err())
	}
	points := stream.Msg().GetPoints()
	if len(points) != 1 {
		t.Fatalf("got %d points, want 1 (it would be missing if stored an hour ago)", len(points))
	}
	p := points[0]
	if p.GetAgentId() != "dads-laptop" || p.GetDirection() != netwatchv1.Direction_DIRECTION_UPLOAD || p.GetBytesPerSec() != 1_250_000 {
		t.Errorf("point = %v", p)
	}
	if age := time.Since(p.GetAt().AsTime()); age < 0 || age > 5*time.Second {
		t.Errorf("sample is %v old, want about 1s", age)
	}
}
