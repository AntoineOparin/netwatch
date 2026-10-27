// Package incident finds lag incidents in the probe data and names the
// computers that were busy at the same time.
//
// The Detector is pure: feed it samples in the order they're stored, and it
// decides only from the samples' own timestamps, never the wall clock. That
// makes it easy to test with a scripted lag spike.
package incident

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/collector/hostnet"
	"github.com/AntoineOparin/netwatch/internal/collector/probe"
	"github.com/AntoineOparin/netwatch/internal/collector/procnet"
)

type Location int

const (
	LocationUnknown Location = iota
	// The router was slow too: Wi-Fi or the local network.
	LocationLocal
	// The router was fine: the connection to the ISP or beyond.
	LocationUpstream
)

// Suspect is a computer that was busy during an incident. Rates are peaks
// (bytes/sec) rather than averages: lag often comes from the first burst of
// an upload, and an average over a few seconds would hide it.
type Suspect struct {
	Device      string  `json:"device"`
	PeakUpBps   float64 `json:"peak_up_bps"`
	PeakDownBps float64 `json:"peak_down_bps"`
	// The app that moved the most data in the suspect's busy direction,
	// if the agent reports apps (macOS).
	TopApp string `json:"top_app,omitempty"`
}

type Incident struct {
	ID         int64
	Start      time.Time
	End        time.Time // zero while ongoing
	Location   Location
	BaselineMS float64 // normal internet round trip before it started
	PeakMS     float64
	Lost       int // lost pings during the incident
	Probes     int // pings during the incident
	Suspects   []Suspect
	// Final is set once suspects are settled; the incident won't change again.
	Final bool
	// Outage: the internet stopped answering entirely for a while (at least
	// OutageAfter pings lost in a row), not just slowed down.
	Outage bool
}

type Config struct {
	// How much history the baseline (median round trip) is taken from.
	BaselineWindow time.Duration
	// No baseline until this many pings have been seen.
	MinBaselineSamples int
	// Lag means at least this much above baseline.
	MinRiseMS float64
	// An incident ends after this long with no lag.
	EndAfter time.Duration
	// How long after an incident ends to wait for late agent reports
	// before naming suspects for good.
	Grace time.Duration
	// Traffic counted from this long before the incident started: the
	// upload begins slightly before the queue fills and pings slow down.
	Lead time.Duration
	// Below these peak rates a computer isn't considered busy.
	MinUpBps, MinDownBps float64
	// At most this many suspects per incident.
	MaxSuspects int
	// This many internet pings lost in a row make an incident an outage.
	OutageAfter int
	// A silence longer than this between internet pings means the hub
	// wasn't measuring (the computer slept, the hub restarted). An open
	// incident is closed at the gap rather than stretched across it.
	MaxGap time.Duration
}

func DefaultConfig() Config {
	return Config{
		BaselineWindow:     10 * time.Minute,
		MinBaselineSamples: 30,
		MinRiseMS:          30,
		EndAfter:           5 * time.Second,
		// Per-app samples reach the hub up to ~5 s late (sampling interval
		// plus report interval), so wait a bit longer than that.
		Grace:       8 * time.Second,
		Lead:        3 * time.Second,
		MinUpBps:    125_000, // 1 Mbps
		MinDownBps:  625_000, // 5 Mbps
		MaxSuspects: 3,
		OutageAfter: 5,
		MaxGap:      10 * time.Second,
	}
}

type point struct {
	at time.Time
	v  float64 // NaN = lost ping
}

type Detector struct {
	cfg      Config
	internet []point
	gateway  []point
	traffic  map[string]map[string][]point // device -> metric -> rates
	apps     map[string]map[string][]point // device -> metric|app -> rates

	open *Incident
	// Last time the smoothed signal said "lagging" (decides when it's over),
	// and the last ping that was itself slow or lost (becomes End).
	lastBad, lastSlowPing time.Time
	// Closed, but suspects wait for late agent reports until End+Grace.
	pending *Incident
	// Internet pings lost in a row, right now.
	lostRun int
	// When the previous internet ping was sent.
	lastInternet time.Time
}

func NewDetector(cfg Config) *Detector {
	return &Detector{cfg: cfg, traffic: map[string]map[string][]point{}, apps: map[string]map[string][]point{}}
}

// Observe takes one stored sample and returns every incident that started,
// changed or ended because of it. Returned incidents stay owned by the
// Detector; the caller may set ID (when saving) but nothing else.
func (d *Detector) Observe(s collector.Sample) []*Incident {
	switch {
	case s.Metric == hostnet.MetricRx || s.Metric == hostnet.MetricTx:
		byMetric := d.traffic[s.Device]
		if byMetric == nil {
			byMetric = map[string][]point{}
			d.traffic[s.Device] = byMetric
		}
		byMetric[s.Metric] = d.keep(append(byMetric[s.Metric], point{s.At, s.Value}), s.At)
		return nil
	case s.Metric == procnet.MetricRx || s.Metric == procnet.MetricTx:
		byApp := d.apps[s.Device]
		if byApp == nil {
			byApp = map[string][]point{}
			d.apps[s.Device] = byApp
		}
		k := s.Metric + "|" + s.Labels["app"]
		byApp[k] = d.keep(append(byApp[k], point{s.At, s.Value}), s.At)
		return nil
	case s.Metric == probe.MetricRTT && s.Device == "hub":
		switch s.Labels["target"] {
		case probe.TargetGateway:
			d.gateway = d.keep(append(d.gateway, point{s.At, s.Value}), s.At)
		case probe.TargetInternet:
			var interrupted []*Incident
			if !d.lastInternet.IsZero() && s.At.Sub(d.lastInternet) > d.cfg.MaxGap {
				interrupted = d.interrupt()
			}
			d.lastInternet = s.At
			d.internet = d.keep(append(d.internet, point{s.At, s.Value}), s.At)
			if math.IsNaN(s.Value) {
				d.lostRun++
			} else {
				d.lostRun = 0
			}
			return append(interrupted, d.evaluate(s.At)...)
		}
	}
	return nil
}

// keep drops points older than the baseline window.
func (d *Detector) keep(points []point, now time.Time) []point {
	cutoff := now.Add(-d.cfg.BaselineWindow)
	i := 0
	for i < len(points) && points[i].at.Before(cutoff) {
		i++
	}
	return points[i:]
}

// interrupt handles a gap in measurement: an open incident ends at its last
// slow ping before the gap and is finalized with what was known then, and so
// is one still waiting out its grace period.
func (d *Detector) interrupt() []*Incident {
	var changed []*Incident
	if p := d.pending; p != nil {
		p.Suspects = d.suspects(p.Start, p.End)
		p.Final = true
		changed = append(changed, p)
		d.pending = nil
	}
	if inc := d.open; inc != nil {
		d.update(inc, d.lastSlowPing)
		inc.End = d.lastSlowPing
		inc.Final = true
		changed = append(changed, inc)
		d.open = nil
	}
	d.lostRun = 0
	return changed
}

// evaluate runs once per internet ping.
func (d *Detector) evaluate(now time.Time) []*Incident {
	var changed []*Incident
	if p := d.pending; p != nil && !now.Before(p.End.Add(d.cfg.Grace)) {
		p.Suspects = d.suspects(p.Start, p.End)
		p.Final = true
		changed = append(changed, p)
		d.pending = nil
	}

	if d.open != nil {
		// While open, compare against the baseline from before it started,
		// so a long incident doesn't drag its own baseline up.
		if d.isSlow(d.internet[len(d.internet)-1], d.open.BaselineMS) {
			d.lastSlowPing = now
		}
		if d.isLagging(d.open.BaselineMS) {
			d.lastBad = now
		} else if now.Sub(d.lastBad) >= d.cfg.EndAfter {
			d.update(d.open, d.lastSlowPing)
			d.open.End = d.lastSlowPing
			changed = append(changed, d.open)
			d.pending, d.open = d.open, nil
			return changed
		}
		d.update(d.open, now)
		return append(changed, d.open)
	}

	baseline, ok := d.baseline(d.internet, time.Time{})
	if !ok || !d.isLagging(baseline) {
		return changed
	}
	// It started at the first of the recent pings that was itself slow.
	start := now
	for _, p := range d.internet[len(d.internet)-5:] {
		if d.isSlow(p, baseline) {
			start = p.at
			break
		}
	}
	d.open = &Incident{Start: start, BaselineMS: baseline}
	d.lastBad, d.lastSlowPing = now, now
	d.update(d.open, now)
	return append(changed, d.open)
}

// isLagging looks at the last few pings: the median of the last 3 well
// above baseline (so one stray spike doesn't count), or 2 of the last 5 lost.
func (d *Detector) isLagging(baseline float64) bool {
	n := len(d.internet)
	if n < 5 {
		return false
	}
	lost := 0
	for _, p := range d.internet[n-5:] {
		if math.IsNaN(p.v) {
			lost++
		}
	}
	if lost >= 2 {
		return true
	}
	return median(d.internet[n-3:], math.Inf(1)) > d.threshold(baseline)
}

// isSlow judges a single ping: lost, or well above baseline.
func (d *Detector) isSlow(p point, baseline float64) bool {
	return math.IsNaN(p.v) || p.v > d.threshold(baseline)
}

// threshold is how slow counts as lag.
func (d *Detector) threshold(baseline float64) float64 {
	return baseline + d.cfg.MinRiseMS
}

// baseline is the median round trip of the window (before `before`, if set).
func (d *Detector) baseline(points []point, before time.Time) (float64, bool) {
	var vs []point
	for _, p := range points {
		if !before.IsZero() && !p.at.Before(before) {
			break
		}
		if !math.IsNaN(p.v) {
			vs = append(vs, p)
		}
	}
	if len(vs) < d.cfg.MinBaselineSamples {
		return 0, false
	}
	return median(vs, 0), true
}

// update refreshes everything about an incident that can be computed from
// the pings between its start and `to`.
func (d *Detector) update(inc *Incident, to time.Time) {
	if d.lostRun >= d.cfg.OutageAfter {
		inc.Outage = true
	}
	inc.PeakMS, inc.Lost, inc.Probes = 0, 0, 0
	for _, p := range between(d.internet, inc.Start, to) {
		inc.Probes++
		if math.IsNaN(p.v) {
			inc.Lost++
		} else {
			inc.PeakMS = max(inc.PeakMS, p.v)
		}
	}
	inc.Location = d.location(inc.Start, to)
	// Provisional until the grace period after the end has passed.
	inc.Suspects = d.suspects(inc.Start, to)
}

// location decides whether the router was slow too.
func (d *Detector) location(from, to time.Time) Location {
	base, ok := d.baseline(d.gateway, from)
	during := between(d.gateway, from, to)
	if !ok || len(during) == 0 {
		return LocationUnknown
	}
	lost := 0
	for _, p := range during {
		if math.IsNaN(p.v) {
			lost++
		}
	}
	if float64(lost)/float64(len(during)) >= 0.2 ||
		median(during, math.Inf(1)) > d.threshold(base) {
		return LocationLocal
	}
	return LocationUpstream
}

// suspects lists the computers that were busy from just before `from`
// until `to`, busiest uploader first.
func (d *Detector) suspects(from, to time.Time) []Suspect {
	from = from.Add(-d.cfg.Lead)
	var out []Suspect
	for device, byMetric := range d.traffic {
		s := Suspect{
			Device:      device,
			PeakUpBps:   peak(between(byMetric[hostnet.MetricTx], from, to)),
			PeakDownBps: peak(between(byMetric[hostnet.MetricRx], from, to)),
		}
		if s.PeakUpBps >= d.cfg.MinUpBps || s.PeakDownBps >= d.cfg.MinDownBps {
			metric := procnet.MetricRx
			if s.PeakUpBps >= d.cfg.MinUpBps {
				metric = procnet.MetricTx
			}
			s.TopApp = d.topApp(device, metric, from, to)
			out = append(out, s)
		}
	}
	// Upload first: on cable internet the upload is the narrow pipe, and a
	// full upload queue delays everyone's packets.
	slices.SortFunc(out, func(a, b Suspect) int {
		if c := cmp.Compare(b.PeakUpBps, a.PeakUpBps); c != 0 {
			return c
		}
		return cmp.Compare(b.PeakDownBps, a.PeakDownBps)
	})
	return out[:min(len(out), d.cfg.MaxSuspects)]
}

// topApp is the app on device with the most data in metric's direction
// between from and to.
func (d *Detector) topApp(device, metric string, from, to time.Time) string {
	var best string
	var bestSum float64
	for k, points := range d.apps[device] {
		m, app, _ := strings.Cut(k, "|")
		if m != metric {
			continue
		}
		var sum float64
		for _, p := range between(points, from, to) {
			sum += p.v
		}
		if sum > bestSum || (sum == bestSum && sum > 0 && app < best) {
			best, bestSum = app, sum
		}
	}
	return best
}

func between(points []point, from, to time.Time) []point {
	var out []point
	for _, p := range points {
		if !p.at.Before(from) && !p.at.After(to) {
			out = append(out, p)
		}
	}
	return out
}

// median of the non-NaN values; ifNone when there are none (all lost).
func median(points []point, ifNone float64) float64 {
	var vs []float64
	for _, p := range points {
		if !math.IsNaN(p.v) {
			vs = append(vs, p.v)
		}
	}
	if len(vs) == 0 {
		return ifNone
	}
	slices.Sort(vs)
	n := len(vs)
	if n%2 == 1 {
		return vs[n/2]
	}
	return (vs[n/2-1] + vs[n/2]) / 2
}

func peak(points []point) float64 {
	m := 0.0
	for _, p := range points {
		if !math.IsNaN(p.v) {
			m = max(m, p.v)
		}
	}
	return m
}
