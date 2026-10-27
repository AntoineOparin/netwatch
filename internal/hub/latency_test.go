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
	"github.com/AntoineOparin/netwatch/internal/collector/discovery"
	"github.com/AntoineOparin/netwatch/internal/collector/hostnet"
	"github.com/AntoineOparin/netwatch/internal/collector/probe"
	"github.com/AntoineOparin/netwatch/internal/collector/procnet"
	"github.com/AntoineOparin/netwatch/internal/collector/wifi"
	"github.com/AntoineOparin/netwatch/internal/devices"
	"github.com/AntoineOparin/netwatch/internal/incident"
	"github.com/AntoineOparin/netwatch/internal/series"
)

type testHub struct {
	store     *series.Store
	devices   *devices.Store
	incidents *incident.Store
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
	incidents, err := incident.OpenStore(store.DB())
	if err != nil {
		t.Fatal(err)
	}
	devs, err := devices.OpenStore(store.DB())
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	srv := httptest.NewServer(NewServer(Config{
		Registry:           reg,
		Store:              store,
		Incidents:          incidents,
		Devices:            devs,
		ReportInterval:     time.Second,
		DeviceOnlineWindow: 3 * time.Minute,
		Log:                slog.New(slog.NewTextHandler(io.Discard, nil)),
	}).Handler(http.NotFoundHandler()))
	t.Cleanup(srv.Close)
	return &testHub{
		store:     store,
		devices:   devs,
		incidents: incidents,
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

func TestListDevices(t *testing.T) {
	h := newTestHub(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now()

	h.registry.Upsert(AgentInfo{ID: "dads-laptop", Hostname: "Dads-Laptop", IPs: []string{"192.168.0.20"}})
	for _, o := range []discovery.Observation{
		{MAC: "aa:00:00:00:00:10", IP: "192.168.0.10", Hostname: "old-tv", At: now.Add(-time.Hour)},
		{MAC: "7c:ed:8d:00:00:02", IP: "192.168.0.9", Vendor: "Microsoft", Replied: true, At: now},
		{MAC: "06:00:00:00:00:20", IP: "192.168.0.20", Private: true, Hostname: "Dads-Laptop", At: now},
	} {
		if err := h.devices.Seen(o); err != nil {
			t.Fatal(err)
		}
	}
	for i, v := range []float64{4, 8, math.NaN(), 4} {
		h.append(t, collector.Sample{Device: "7c:ed:8d:00:00:02", Metric: discovery.MetricLANRTT, Value: v, At: now.Add(time.Duration(i-4) * 5 * time.Second)})
	}

	if _, err := h.dashboard.SetDeviceNickname(ctx, connect.NewRequest(&netwatchv1.SetDeviceNicknameRequest{Mac: "7c:ed:8d:00:00:02", Nickname: "  Xbox "})); err != nil {
		t.Fatal(err)
	}
	resp, err := h.dashboard.ListDevices(ctx, connect.NewRequest(&netwatchv1.ListDevicesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetDevices()
	if len(got) != 3 {
		t.Fatalf("got %d devices, want 3", len(got))
	}
	// Online first, sorted numerically (.9 before .20), then the offline TV.
	if got[0].GetIp() != "192.168.0.9" || got[1].GetIp() != "192.168.0.20" || got[2].GetOnline() {
		t.Fatalf("order = %s, %s, %s (online %v)", got[0].GetIp(), got[1].GetIp(), got[2].GetIp(), got[2].GetOnline())
	}
	xbox := got[0]
	if xbox.GetNickname() != "Xbox" || !xbox.GetAnswersPing() || xbox.GetPings() != 4 {
		t.Errorf("xbox = %v", xbox)
	}
	if xbox.GetPingAvgMs() != 16.0/3 || xbox.GetPingJitterMs() != 4 || xbox.GetPingLoss() != 0.25 {
		t.Errorf("ping stats avg %v jitter %v loss %v, want 5.33, 4, 0.25", xbox.GetPingAvgMs(), xbox.GetPingJitterMs(), xbox.GetPingLoss())
	}
	if got[1].GetAgentId() != "dads-laptop" {
		t.Errorf("device at the agent's IP should link to it, got %q", got[1].GetAgentId())
	}
}

func TestListTopApps(t *testing.T) {
	h := newTestHub(t)
	now := time.Now()
	chrome := map[string]string{"app": "Google Chrome"}
	onedrive := map[string]string{"app": "OneDrive"}
	// Two 2-second samples of Chrome downloading 1 MB/s = 4 MB in the minute;
	// one of OneDrive uploading 3 MB/s = 6 MB.
	h.append(t, collector.Sample{Device: "mac", Metric: procnet.MetricRx, Labels: chrome, Value: 1e6, At: now.Add(-10 * time.Second)})
	h.append(t, collector.Sample{Device: "mac", Metric: procnet.MetricRx, Labels: chrome, Value: 1e6, At: now.Add(-8 * time.Second)})
	h.append(t, collector.Sample{Device: "mac", Metric: procnet.MetricTx, Labels: onedrive, Value: 3e6, At: now.Add(-6 * time.Second)})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := h.dashboard.ListTopApps(ctx, connect.NewRequest(&netwatchv1.ListTopAppsRequest{Window: durationpb.New(time.Minute)}))
	if err != nil {
		t.Fatal(err)
	}
	apps := resp.Msg.GetApps()
	if len(apps) != 2 || apps[0].GetApp() != "OneDrive" || apps[1].GetApp() != "Google Chrome" {
		t.Fatalf("apps = %v, want OneDrive then Chrome", apps)
	}
	if got := apps[0].GetAvgUploadBytesPerSec(); got != 6e6/60 {
		t.Errorf("OneDrive avg upload = %v, want 6 MB over 60 s", got)
	}
	if got := apps[1].GetPeakDownloadBytesPerSec(); got != 1e6 {
		t.Errorf("Chrome peak download = %v, want 1 MB/s", got)
	}
}

func TestListTopAppsSkipsChatter(t *testing.T) {
	h := newTestHub(t)
	// 2 s at 50 KB/s = 100 KB over a minute: ~1.7 KB/s average, below the floor.
	h.append(t, collector.Sample{Device: "mac", Metric: procnet.MetricTx, Labels: map[string]string{"app": "apsd"}, Value: 50_000, At: time.Now().Add(-5 * time.Second)})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := h.dashboard.ListTopApps(ctx, connect.NewRequest(&netwatchv1.ListTopAppsRequest{Window: durationpb.New(time.Minute)}))
	if err != nil {
		t.Fatal(err)
	}
	if apps := resp.Msg.GetApps(); len(apps) != 0 {
		t.Fatalf("apps = %v, want background chatter left out", apps)
	}
}

func TestGetUptime(t *testing.T) {
	h := newTestHub(t)
	now := time.Now()
	internet := map[string]string{"target": "internet"}
	for i := range 120 { // two minutes of watching
		h.append(t, collector.Sample{Device: "hub", Metric: probe.MetricRTT, Labels: internet, Value: 20, At: now.Add(-time.Duration(120-i) * time.Second)})
	}
	if err := h.incidents.Save(&incident.Incident{Start: now.Add(-50 * time.Second), End: now.Add(-38 * time.Second), Outage: true}, now); err != nil {
		t.Fatal(err)
	}
	if err := h.incidents.Save(&incident.Incident{Start: now.Add(-20 * time.Second), End: now.Add(-10 * time.Second)}, now); err != nil { // lag, not an outage
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := h.dashboard.GetUptime(ctx, connect.NewRequest(&netwatchv1.GetUptimeRequest{Window: durationpb.New(time.Hour)}))
	if err != nil {
		t.Fatal(err)
	}
	m := resp.Msg
	if m.GetMonitored().AsDuration() != 2*time.Minute || m.GetOutages() != 1 || m.GetDowntime().AsDuration() != 12*time.Second || m.GetLongestOutage().AsDuration() != 12*time.Second {
		t.Fatalf("uptime = %v, want 2m watched, one 12 s outage", m)
	}
}

func TestGetDevicePingAndWifi(t *testing.T) {
	h := newTestHub(t)
	now := time.Now()
	h.append(t, collector.Sample{Device: "cc:b0:b3:00:00:01", Metric: discovery.MetricLANRTT, Value: 13, At: now.Add(-10 * time.Second)})
	h.append(t, collector.Sample{Device: "cc:b0:b3:00:00:01", Metric: discovery.MetricLANRTT, Value: math.NaN(), At: now.Add(-5 * time.Second)})
	h.append(t, collector.Sample{Device: "someone-else", Metric: discovery.MetricLANRTT, Value: 2, At: now.Add(-5 * time.Second)})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := h.dashboard.GetDevicePing(ctx, connect.NewRequest(&netwatchv1.GetDevicePingRequest{Mac: "cc:b0:b3:00:00:01"}))
	if err != nil {
		t.Fatal(err)
	}
	if pts := resp.Msg.GetPoints(); len(pts) != 2 || pts[0].GetRttMs() != 13 || !pts[1].GetLost() {
		t.Fatalf("points = %v, want 13 ms then lost", pts)
	}

	h.registry.Upsert(AgentInfo{ID: "mac", Hostname: "Mac"})
	h.append(t, collector.Sample{Device: "mac", Metric: wifi.MetricSignal, Value: -68, At: now})
	h.append(t, collector.Sample{Device: "mac", Metric: wifi.MetricBand, Value: 5, At: now})
	h.append(t, collector.Sample{Device: "mac", Metric: wifi.MetricChannel, Value: 153, At: now})
	agents, err := h.dashboard.ListAgents(ctx, connect.NewRequest(&netwatchv1.ListAgentsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	w := agents.Msg.GetAgents()[0].GetWifi()
	if w.GetSignalDbm() != -68 || w.GetBandGhz() != 5 || w.GetChannel() != 153 {
		t.Fatalf("wifi = %v, want -68 dBm on 5 GHz channel 153", w)
	}
}
