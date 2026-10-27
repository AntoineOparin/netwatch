package hub

import (
	"context"
	"math"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/internal/collector/hostnet"
)

var directions = map[string]netwatchv1.Direction{
	hostnet.MetricRx: netwatchv1.Direction_DIRECTION_DOWNLOAD,
	hostnet.MetricTx: netwatchv1.Direction_DIRECTION_UPLOAD,
}

func (s *Server) WatchThroughput(
	ctx context.Context,
	req *connect.Request[netwatchv1.WatchThroughputRequest],
	stream *connect.ServerStream[netwatchv1.WatchThroughputResponse],
) error {
	return watch(ctx, s.cfg.Store, []string{hostnet.MetricRx, hostnet.MetricTx}, req.Msg.GetHistory().AsDuration(),
		throughputPoint,
		func(points []*netwatchv1.ThroughputPoint) error {
			return stream.Send(&netwatchv1.WatchThroughputResponse{Points: points})
		},
	)
}

func throughputPoint(device, metric string, labels map[string]string, at time.Time, bps float64) *netwatchv1.ThroughputPoint {
	if math.IsNaN(bps) {
		return nil
	}
	return &netwatchv1.ThroughputPoint{
		AgentId:     device,
		Direction:   directions[metric],
		At:          timestamppb.New(at),
		BytesPerSec: bps,
		Interface:   labels["iface"],
	}
}
