// Package wifi reports this computer's Wi-Fi link: signal strength, noise,
// link speed, band and channel. A weak signal is the most common reason a
// device on Wi-Fi lags while the internet itself is fine.
//
// Each OS exposes this differently, all without admin rights:
//
//	macOS    system_profiler SPAirPortDataType -json
//	Windows  netsh wlan show interfaces (English or French output)
//	Linux    /proc/net/wireless (signal only)
package wifi

import (
	"bufio"
	"context"
	"encoding/json"
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

var (
	dbmPair   = regexp.MustCompile(`(-?\d+)\s*dBm\s*/\s*(-?\d+)\s*dBm`)
	chanBand  = regexp.MustCompile(`^(\d+)\s*\((\d+(?:\.\d+)?)\s*GHz`)
	firstNumb = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)
)

// parseDarwin reads `system_profiler SPAirPortDataType -json`. The current
// network's name is redacted without location permission, but the signal
// numbers aren't.
func parseDarwin(data []byte) (reading, error) {
	var doc struct {
		Items []struct {
			Interfaces []struct {
				Current *struct {
					SignalNoise string `json:"spairport_signal_noise"`
					Channel     string `json:"spairport_network_channel"`
					Rate        any    `json:"spairport_network_rate"`
				} `json:"spairport_current_network_information"`
			} `json:"spairport_airport_interfaces"`
		} `json:"SPAirPortDataType"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return reading{}, err
	}
	for _, item := range doc.Items {
		for _, ifc := range item.Interfaces {
			cur := ifc.Current
			if cur == nil || cur.SignalNoise == "" {
				continue
			}
			var r reading
			if m := dbmPair.FindStringSubmatch(cur.SignalNoise); m != nil {
				r.signalDBm, _ = strconv.ParseFloat(m[1], 64)
				r.noiseDBm, _ = strconv.ParseFloat(m[2], 64)
			}
			if m := chanBand.FindStringSubmatch(cur.Channel); m != nil {
				r.channel, _ = strconv.Atoi(m[1])
				r.bandGHz, _ = strconv.ParseFloat(m[2], 64)
			}
			switch v := cur.Rate.(type) {
			case float64:
				r.txRateMbps = v
			case string:
				r.txRateMbps, _ = strconv.ParseFloat(v, 64)
			}
			if r.signalDBm != 0 {
				return r, nil
			}
		}
	}
	return reading{}, errNotConnected
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
