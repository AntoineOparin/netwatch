package incident

import (
	"math"
	"testing"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/collector/hostnet"
	"github.com/AntoineOparin/netwatch/internal/collector/probe"
)

var t0 = time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)

// scenario replays one second at a time: router ping, internet ping, and
// each computer's upload rate.
type second struct {
	gateway, internet float64
	upload            map[string]float64 // device -> bytes/sec
}

func run(d *Detector, secs []second) (all []*Incident) {
	for i, s := range secs {
		at := t0.Add(time.Duration(i) * time.Second)
		for device, up := range s.upload {
			d.Observe(collector.Sample{Device: device, Metric: hostnet.MetricTx, Value: up, At: at})
			d.Observe(collector.Sample{Device: device, Metric: hostnet.MetricRx, Value: 0, At: at})
		}
		d.Observe(collector.Sample{Device: "hub", Metric: probe.MetricRTT, Labels: map[string]string{"target": probe.TargetGateway}, Value: s.gateway, At: at})
		for _, inc := range d.Observe(collector.Sample{Device: "hub", Metric: probe.MetricRTT, Labels: map[string]string{"target": probe.TargetInternet}, Value: s.internet, At: at}) {
			if len(all) == 0 || all[len(all)-1] != inc {
				all = append(all, inc)
			}
		}
	}
	return all
}

func repeat(n int, s second) []second {
	out := make([]second, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func concat(parts ...[]second) []second {
	var out []second
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

const mbps = 125_000.0 // bytes/sec in one megabit/sec

func TestUploadCausesUpstreamIncident(t *testing.T) {
	quiet := map[string]float64{"dads-laptop": 0.1 * mbps, "my-mac": 0.2 * mbps}
	uploading := map[string]float64{"dads-laptop": 9 * mbps, "my-mac": 0.2 * mbps}

	got := run(NewDetector(DefaultConfig()), concat(
		repeat(60, second{gateway: 4, internet: 20, upload: quiet}),
		repeat(3, second{gateway: 4, internet: 20, upload: uploading}),   // upload starts...
		repeat(20, second{gateway: 5, internet: 180, upload: uploading}), // ...queue fills
		repeat(15, second{gateway: 4, internet: 20, upload: quiet}),
	))

	if len(got) != 1 {
		t.Fatalf("got %d incidents, want 1", len(got))
	}
	inc := got[0]
	if inc.End.IsZero() {
		t.Fatal("incident never ended")
	}
	if inc.Location != LocationUpstream {
		t.Errorf("location = %v, want upstream (router stayed fast)", inc.Location)
	}
	if inc.BaselineMS != 20 || inc.PeakMS != 180 {
		t.Errorf("baseline %v peak %v, want 20 and 180", inc.BaselineMS, inc.PeakMS)
	}
	if d := inc.End.Sub(inc.Start); d < 18*time.Second || d > 22*time.Second {
		t.Errorf("duration %v, want about 20s", d)
	}
	if len(inc.Suspects) != 1 || inc.Suspects[0].Device != "dads-laptop" {
		t.Fatalf("suspects = %+v, want only dads-laptop", inc.Suspects)
	}
	if up := inc.Suspects[0].PeakUpBps; up != 9*mbps {
		t.Errorf("suspect peak upload %v Mbps, want 9", up/mbps)
	}
	if !inc.Final {
		t.Error("incident should be final after the grace period")
	}
}

func TestSlowRouterMeansLocalProblem(t *testing.T) {
	got := run(NewDetector(DefaultConfig()), concat(
		repeat(60, second{gateway: 4, internet: 20}),
		repeat(10, second{gateway: 150, internet: 170}),
		repeat(10, second{gateway: 4, internet: 20}),
	))
	if len(got) != 1 || got[0].Location != LocationLocal {
		t.Fatalf("got %+v, want one local incident", got)
	}
	if len(got[0].Suspects) != 0 {
		t.Errorf("suspects = %+v, want none: nobody was busy", got[0].Suspects)
	}
}

func TestSingleSpikeIsNotAnIncident(t *testing.T) {
	secs := repeat(90, second{gateway: 4, internet: 20})
	secs[70].internet = 400
	if got := run(NewDetector(DefaultConfig()), secs); len(got) != 0 {
		t.Fatalf("got %d incidents from one stray spike, want 0", len(got))
	}
}

func TestPacketLossIsAnIncident(t *testing.T) {
	secs := repeat(90, second{gateway: 4, internet: 20})
	for i := 70; i < 76; i += 2 {
		secs[i].internet = math.NaN()
	}
	got := run(NewDetector(DefaultConfig()), secs)
	if len(got) != 1 || got[0].Lost != 3 {
		t.Fatalf("got %+v, want one incident with 3 lost pings", got)
	}
}

func TestNoBaselineNoIncident(t *testing.T) {
	// Lag right after startup: there's nothing normal to compare against yet.
	if got := run(NewDetector(DefaultConfig()), repeat(10, second{gateway: 4, internet: 300})); len(got) != 0 {
		t.Fatalf("got %d incidents without a baseline, want 0", len(got))
	}
}
