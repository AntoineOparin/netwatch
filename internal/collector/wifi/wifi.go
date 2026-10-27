// Package wifi reports this computer's Wi-Fi link: signal strength, noise,
// link speed, band and channel. A weak signal is the most common reason a
// device on Wi-Fi lags while the internet itself is fine.
//
// Each OS exposes this differently, all without admin rights:
//
//	macOS    CoreWLAN (needs cgo; see wifi_darwin_cgo.go)
//	Windows  netsh wlan show interfaces (English or French output)
//	Linux    /proc/net/wireless (signal only)
//
// Whatever is used must not scan for networks: a scan takes the radio off
// the current channel and causes the very lag spikes NetWatch measures.
package wifi

import (
	"bufio"
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

const (
	MetricSignal  = "wifi_signal_dbm"
	MetricNoise   = "wifi_noise_dbm"
	MetricTxRate  = "wifi_tx_rate_mbps"
	MetricBand    = "wifi_band_ghz"
	MetricChannel = "wifi_channel"
)

// errNotConnected means there's no Wi-Fi link right now (wired, Wi-Fi off,
// or no Wi-Fi hardware). It isn't a failure; there's just nothing to report.
var errNotConnected = errors.New("not connected to Wi-Fi")

// reading is one look at the Wi-Fi link. Zero means "not reported".
type reading struct {
	signalDBm  float64
	noiseDBm   float64
	txRateMbps float64
	bandGHz    float64
	channel    int
}

type Collector struct {
	interval time.Duration
	read     func(context.Context) (reading, error)
}

func New(interval time.Duration) *Collector {
	return &Collector{interval: interval, read: read}
}

func (c *Collector) Name() string { return "wifi" }

func (c *Collector) Run(ctx context.Context, out chan<- collector.Sample) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		r, err := c.read(ctx)
		switch {
		case ctx.Err() != nil:
			return nil
		case errors.Is(err, errNotConnected):
		case err != nil:
			return err
		default:
			for _, s := range samples(r, time.Now()) {
				select {
				case out <- s:
				case <-ctx.Done():
					return nil
				}
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func samples(r reading, at time.Time) []collector.Sample {
	var out []collector.Sample
	add := func(metric string, v float64) {
		if v != 0 {
			out = append(out, collector.Sample{Source: "wifi", Metric: metric, Value: v, At: at})
		}
	}
	add(MetricSignal, r.signalDBm)
	add(MetricNoise, r.noiseDBm)
	add(MetricTxRate, r.txRateMbps)
	add(MetricBand, r.bandGHz)
	add(MetricChannel, float64(r.channel))
	return out
}

var firstNumb = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)

// bandGHz turns CoreWLAN's CWChannelBand into GHz.
func bandGHz(band int) float64 {
	switch band {
	case 1:
		return 2.4
	case 2:
		return 5
	case 3:
		return 6
	}
	return 0
}

// parseWindows reads `netsh wlan show interfaces`. Labels are localized, so
// both English and French are recognized. Windows reports signal as a
// percentage; its documented mapping is linear from -100 dBm (0%) to
// -50 dBm (100%).
func parseWindows(out string) (reading, error) {
	var r reading
	var haveSignal bool
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		key, value, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		num := strings.ReplaceAll(firstNumb.FindString(value), ",", ".")
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			continue
		}
		switch {
		case key == "signal":
			r.signalDBm = v/2 - 100
			haveSignal = true
		case key == "channel" || key == "canal":
			r.channel = int(v)
		case key == "band" || key == "bande":
			r.bandGHz = v
		case strings.HasPrefix(key, "transmit") || strings.HasPrefix(key, "transmission"):
			r.txRateMbps = v
		}
	}
	if !haveSignal {
		return reading{}, errNotConnected
	}
	return r, nil
}

// parseLinux reads /proc/net/wireless:
//
//	Inter-| sta-|   Quality        |   Discarded packets  ...
//	 face | tus | link level noise |  nwid  crypt   frag  ...
//	 wlan0: 0000   54.  -56.  -256        0      0      0 ...
//
// Level is the signal in dBm; a noise of -256 means "not reported".
func parseLinux(out string) (reading, error) {
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok || strings.ContainsAny(name, "|") {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 4 {
			continue
		}
		level, err := strconv.ParseFloat(strings.TrimSuffix(f[2], "."), 64)
		if err != nil || level == 0 {
			continue
		}
		r := reading{signalDBm: level}
		if noise, err := strconv.ParseFloat(strings.TrimSuffix(f[3], "."), 64); err == nil && noise > -256 {
			r.noiseDBm = noise
		}
		return r, nil
	}
	return reading{}, errNotConnected
}
