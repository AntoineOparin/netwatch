package hub

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"time"

	"connectrpc.com/connect"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/internal/collector/procnet"
)

const (
	defaultAppWindow = time.Minute
	maxAppsPerAgent  = 5
	// Apps averaging less than this (0.05 Mbps) over the window are left
	// out: a short burst of background chatter isn't worth listing.
	minAvgBytesPerSec = 6_250
)

func (s *Server) ListTopApps(
	_ context.Context,
	req *connect.Request[netwatchv1.ListTopAppsRequest],
) (*connect.Response[netwatchv1.ListTopAppsResponse], error) {
	window := req.Msg.GetWindow().AsDuration()
	if window <= 0 {
		window = defaultAppWindow
	}
	since := time.Now().Add(-window)

	type key struct{ agent, app string }
	usage := map[key]*netwatchv1.AppUsage{}
	for metric, down := range map[string]bool{procnet.MetricRx: true, procnet.MetricTx: false} {
		series, err := s.cfg.Store.Query(metric, since)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, ser := range series {
			k := key{ser.Device, ser.Labels["app"]}
			u := usage[k]
			if u == nil {
				u = &netwatchv1.AppUsage{AgentId: k.agent, App: k.app}
				usage[k] = u
			}
			// Agents only report an app while it's busy, one rate per
			// procnet.Interval; quiet intervals are missing, i.e. zero.
			var bytes, peak float64
			for _, p := range ser.Points {
				bytes += p.Value * procnet.Interval.Seconds()
				peak = max(peak, p.Value)
			}
			avg := bytes / window.Seconds()
			if down {
				u.AvgDownloadBytesPerSec, u.PeakDownloadBytesPerSec = avg, peak
			} else {
				u.AvgUploadBytesPerSec, u.PeakUploadBytesPerSec = avg, peak
			}
		}
	}

	byAgent := map[string][]*netwatchv1.AppUsage{}
	for k, u := range usage {
		if u.AvgDownloadBytesPerSec+u.AvgUploadBytesPerSec >= minAvgBytesPerSec {
			byAgent[k.agent] = append(byAgent[k.agent], u)
		}
	}
	var out []*netwatchv1.AppUsage
	for _, agent := range slices.Sorted(maps.Keys(byAgent)) {
		apps := byAgent[agent]
		slices.SortFunc(apps, func(a, b *netwatchv1.AppUsage) int {
			total := func(u *netwatchv1.AppUsage) float64 {
				return u.AvgDownloadBytesPerSec + u.AvgUploadBytesPerSec
			}
			if c := cmp.Compare(total(b), total(a)); c != 0 {
				return c
			}
			return cmp.Compare(a.App, b.App)
		})
		out = append(out, apps[:min(len(apps), maxAppsPerAgent)]...)
	}
	return connect.NewResponse(&netwatchv1.ListTopAppsResponse{Apps: out}), nil
}
