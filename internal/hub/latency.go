package hub

import (
	"context"
	"math"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/internal/collector/probe"
)

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
	return watch(ctx, s.cfg.Store, []string{probe.MetricRTT}, req.Msg.GetHistory().AsDuration(),
		latencyPoint,
		func(points []*netwatchv1.LatencyPoint) error {
			return stream.Send(&netwatchv1.WatchLatencyResponse{Points: points})
		},
	)
}

// latencyPoint converts a stored probe sample (NaN = lost) to the wire
// format. Only the hub's own probes count.
func latencyPoint(device, _ string, labels map[string]string, at time.Time, rttMS float64) *netwatchv1.LatencyPoint {
	if device != "hub" {
		return nil
	}
	pt := &netwatchv1.LatencyPoint{
		Target: latencyTargets[labels["target"]],
		Addr:   labels["addr"],
		At:     timestamppb.New(at),
	}
	if math.IsNaN(rttMS) {
		pt.Lost = true
	} else {
		pt.RttMs = rttMS
	}
	return pt
}
