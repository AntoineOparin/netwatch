package hub

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/gen/netwatch/v1/netwatchv1connect"
	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/series"
)

func TestWatchLatencySendsHistoryThenLive(t *testing.T) {
	store := series.NewStore(time.Hour)
	gw := map[string]string{"target": "gateway", "addr": "192.168.0.1"}
	now := time.Now()
	store.Append(collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: 4.5, At: now.Add(-2 * time.Second)})
	store.Append(collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: math.NaN(), At: now.Add(-time.Second)})

	srv := httptest.NewServer(NewServer(NewRegistry(), store, time.Second).Handler(http.NotFoundHandler()))
	defer srv.Close()
	client := netwatchv1connect.NewDashboardServiceClient(srv.Client(), srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := client.WatchLatency(ctx, connect.NewRequest(&netwatchv1.WatchLatencyRequest{
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
	store.Append(collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: math.NaN(), At: now.Add(-time.Second)})
	store.Append(collector.Sample{Device: "hub", Metric: "rtt_ms", Labels: gw, Value: 6, At: now})
	if !stream.Receive() {
		t.Fatalf("no live message: %v", stream.Err())
	}
	if live := stream.Msg().GetPoints(); len(live) != 1 || live[0].GetRttMs() != 6 {
		t.Fatalf("live points = %v, want one point of 6 ms", live)
	}
}
