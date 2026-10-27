package hub

import (
	"context"
	"errors"
	"math"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/internal/collector/discovery"
	"github.com/AntoineOparin/netwatch/internal/collector/probe"
	"github.com/AntoineOparin/netwatch/internal/collector/wifi"
)

const (
	defaultUptimeWindow = 24 * time.Hour
	defaultPingWindow   = time.Hour
	// A Wi-Fi reading older than this is stale (agents send one every 30 s).
	wifiFresh = 2 * time.Minute
)

func (s *Server) GetUptime(
	_ context.Context,
	req *connect.Request[netwatchv1.GetUptimeRequest],
) (*connect.Response[netwatchv1.GetUptimeResponse], error) {
	window := req.Msg.GetWindow().AsDuration()
	if window <= 0 {
		window = defaultUptimeWindow
	}
	now := time.Now()
	since := now.Add(-window)

	// Every internet ping the hub sent is a probe.Interval it was watching,
	// so time the hub itself was off isn't counted as downtime or uptime.
	pings, err := s.cfg.Store.CountPoints(probe.MetricRTT, since, func(device string, labels map[string]string) bool {
		return device == "hub" && labels["target"] == probe.TargetInternet
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	monitored := time.Duration(pings) * probe.Interval

	incidents, err := s.cfg.Incidents.List(since)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var downtime, longest time.Duration
	var outages int32
	for _, inc := range incidents {
		if !inc.Outage {
			continue
		}
		end := inc.End
		if end.IsZero() {
			end = now
		}
		outages++
		longest = max(longest, end.Sub(inc.Start))
		downtime += end.Sub(maxTime(inc.Start, since))
	}
	return connect.NewResponse(&netwatchv1.GetUptimeResponse{
		Monitored:     durationpb.New(monitored),
		Downtime:      durationpb.New(min(downtime, monitored)),
		Outages:       outages,
		LongestOutage: durationpb.New(longest),
	}), nil
}

func (s *Server) GetDevicePing(
	_ context.Context,
	req *connect.Request[netwatchv1.GetDevicePingRequest],
) (*connect.Response[netwatchv1.GetDevicePingResponse], error) {
	mac := req.Msg.GetMac()
	if mac == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("mac is required"))
	}
	window := req.Msg.GetWindow().AsDuration()
	if window <= 0 {
		window = defaultPingWindow
	}
	series, err := s.cfg.Store.Query(discovery.MetricLANRTT, time.Now().Add(-window))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var points []*netwatchv1.PingPoint
	for _, ser := range series {
		if ser.Device != mac {
			continue
		}
		for _, p := range ser.Points {
			pt := &netwatchv1.PingPoint{At: timestamppb.New(p.At)}
			if math.IsNaN(p.Value) {
				pt.Lost = true
			} else {
				pt.RttMs = p.Value
			}
			points = append(points, pt)
		}
	}
	return connect.NewResponse(&netwatchv1.GetDevicePingResponse{Points: points}), nil
}

// latestWifi returns each agent's most recent Wi-Fi reading, if fresh.
func (s *Server) latestWifi() (map[string]*netwatchv1.WifiStatus, error) {
	out := map[string]*netwatchv1.WifiStatus{}
	since := time.Now().Add(-wifiFresh)
	set := map[string]func(*netwatchv1.WifiStatus, float64){
		wifi.MetricSignal:  func(w *netwatchv1.WifiStatus, v float64) { w.SignalDbm = v },
		wifi.MetricNoise:   func(w *netwatchv1.WifiStatus, v float64) { w.NoiseDbm = v },
		wifi.MetricTxRate:  func(w *netwatchv1.WifiStatus, v float64) { w.TxRateMbps = v },
		wifi.MetricBand:    func(w *netwatchv1.WifiStatus, v float64) { w.BandGhz = v },
		wifi.MetricChannel: func(w *netwatchv1.WifiStatus, v float64) { w.Channel = int32(v) },
	}
	for metric, apply := range set {
		series, err := s.cfg.Store.Query(metric, since)
		if err != nil {
			return nil, err
		}
		for _, ser := range series {
			if len(ser.Points) == 0 {
				continue
			}
			last := ser.Points[len(ser.Points)-1]
			w := out[ser.Device]
			if w == nil {
				w = &netwatchv1.WifiStatus{}
				out[ser.Device] = w
			}
			apply(w, last.Value)
			if metric == wifi.MetricSignal {
				w.At = timestamppb.New(last.At)
			}
		}
	}
	// A reading without signal strength isn't worth showing.
	for id, w := range out {
		if w.SignalDbm == 0 {
			delete(out, id)
		}
	}
	return out, nil
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
