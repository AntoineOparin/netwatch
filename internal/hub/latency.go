package hub

import (
	"cmp"
	"context"
	"math"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/collector/probe"
)

const defaultLatencyHistory = 5 * time.Minute

var latencyTargets = map[string]netwatchv1.LatencyTarget{
	probe.TargetGateway:  netwatchv1.LatencyTarget_LATENCY_TARGET_GATEWAY,
	probe.TargetISP:      netwatchv1.LatencyTarget_LATENCY_TARGET_ISP,
	probe.TargetInternet: netwatchv1.LatencyTarget_LATENCY_TARGET_INTERNET,
}

func (s *Server) WatchLatency(
	ctx context.Context,
	req *connect.Request[netwatchv1.WatchLatencyRequest],
	stream *connect.ServerStream[netwatchv1.WatchLatencyResponse],
) error {
	history := req.Msg.GetHistory().AsDuration()
	if history <= 0 {
		history = defaultLatencyHistory
	}

	// Subscribe before reading history so nothing measured in between is
	// missed; anything that shows up in both is skipped below.
	live, unsubscribe := s.store.Subscribe(64)
	defer unsubscribe()

	var points []*netwatchv1.LatencyPoint
	lastSent := map[string]time.Time{} // target label -> newest point sent
	for _, ser := range s.store.Query(probe.MetricRTT, time.Now().Add(-history)) {
		name := ser.Labels["target"]
		for _, pt := range ser.Points {
			points = append(points, latencyPoint(name, ser.Labels["addr"], pt.At, pt.Value))
		}
		if n := len(ser.Points); n > 0 {
			lastSent[name] = ser.Points[n-1].At
		}
	}
	slices.SortFunc(points, func(a, b *netwatchv1.LatencyPoint) int {
		return cmp.Compare(a.GetAt().AsTime().UnixNano(), b.GetAt().AsTime().UnixNano())
	})
	if err := stream.Send(&netwatchv1.WatchLatencyResponse{Points: points}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case smp := <-live:
			if !isLatencySample(smp) {
				continue
			}
			name := smp.Labels["target"]
			if !smp.At.After(lastSent[name]) {
				continue
			}
			lastSent[name] = smp.At
			pt := latencyPoint(name, smp.Labels["addr"], smp.At, smp.Value)
			if err := stream.Send(&netwatchv1.WatchLatencyResponse{Points: []*netwatchv1.LatencyPoint{pt}}); err != nil {
				return err
			}
		}
	}
}

func isLatencySample(smp collector.Sample) bool {
	return smp.Metric == probe.MetricRTT && smp.Device == "hub"
}

// latencyPoint converts a stored sample (NaN = lost) to the wire format.
func latencyPoint(target, addr string, at time.Time, rttMS float64) *netwatchv1.LatencyPoint {
	pt := &netwatchv1.LatencyPoint{
		Target: latencyTargets[target],
		Addr:   addr,
		At:     timestamppb.New(at),
	}
	if math.IsNaN(rttMS) {
		pt.Lost = true
	} else {
		pt.RttMs = rttMS
	}
	return pt
}
