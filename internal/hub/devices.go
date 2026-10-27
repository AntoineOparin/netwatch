package hub

import (
	"cmp"
	"context"
	"errors"
	"math"
	"net"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/internal/collector/discovery"
	"github.com/AntoineOparin/netwatch/internal/devices"
)

const (
	lanPingWindow  = 10 * time.Minute
	maxNicknameLen = 64
)

func (s *Server) ListDevices(
	context.Context,
	*connect.Request[netwatchv1.ListDevicesRequest],
) (*connect.Response[netwatchv1.ListDevicesResponse], error) {
	list, err := s.cfg.Devices.List()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pings, err := s.cfg.Store.Query(discovery.MetricLANRTT, time.Now().Add(-lanPingWindow))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	byMAC := map[string][]float64{}
	for _, ser := range pings {
		for _, p := range ser.Points {
			byMAC[ser.Device] = append(byMAC[ser.Device], p.Value)
		}
	}
	agentByIP := map[string]string{}
	for _, a := range s.cfg.Registry.List() {
		for _, ip := range a.IPs {
			agentByIP[ip] = a.ID
		}
	}

	out := make([]*netwatchv1.LanDevice, 0, len(list))
	for _, d := range list {
		pb := &netwatchv1.LanDevice{
			Mac:         d.MAC,
			Ip:          d.IP,
			Vendor:      d.Vendor,
			PrivateMac:  d.Private,
			Hostname:    d.Hostname,
			Nickname:    d.Nickname,
			FirstSeen:   timestamppb.New(d.FirstSeen),
			LastSeen:    timestamppb.New(d.LastSeen),
			Online:      d.Self || time.Since(d.LastSeen) <= s.cfg.DeviceOnlineWindow,
			AnswersPing: !d.LastReply.IsZero(),
			AgentId:     agentByIP[d.IP],
			IsHub:       d.Self,
			IsRouter:    d.Router,
		}
		if rtts := byMAC[d.MAC]; len(rtts) > 0 {
			pb.PingAvgMs, pb.PingJitterMs, pb.PingLoss = pingStats(rtts)
			pb.Pings = int32(len(rtts))
		}
		out = append(out, pb)
	}
	// Online first, then by address.
	slices.SortStableFunc(out, func(a, b *netwatchv1.LanDevice) int {
		if a.Online != b.Online {
			if a.Online {
				return -1
			}
			return 1
		}
		return cmp.Compare(ipOrder(a.Ip), ipOrder(b.Ip))
	})
	return connect.NewResponse(&netwatchv1.ListDevicesResponse{Devices: out}), nil
}

func (s *Server) SetDeviceNickname(
	_ context.Context,
	req *connect.Request[netwatchv1.SetDeviceNicknameRequest],
) (*connect.Response[netwatchv1.SetDeviceNicknameResponse], error) {
	nickname := strings.TrimSpace(req.Msg.GetNickname())
	if len(nickname) > maxNicknameLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("nickname is too long"))
	}
	err := s.cfg.Devices.SetNickname(req.Msg.GetMac(), nickname)
	if errors.Is(err, devices.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&netwatchv1.SetDeviceNicknameResponse{}), nil
}

// pingStats summarizes round trips (NaN = no answer): average, jitter (mean
// change between consecutive answers) and the share lost.
func pingStats(rtts []float64) (avg, jitter, loss float64) {
	var answered []float64
	for _, v := range rtts {
		if !math.IsNaN(v) {
			answered = append(answered, v)
		}
	}
	loss = float64(len(rtts)-len(answered)) / float64(len(rtts))
	if len(answered) == 0 {
		return 0, 0, loss
	}
	for i, v := range answered {
		avg += v
		if i > 0 {
			jitter += math.Abs(v - answered[i-1])
		}
	}
	avg /= float64(len(answered))
	if len(answered) > 1 {
		jitter /= float64(len(answered) - 1)
	}
	return avg, jitter, loss
}

// ipOrder sorts IPv4 addresses numerically (so .9 comes before .10).
func ipOrder(s string) uint32 {
	ip := net.ParseIP(s).To4()
	if ip == nil {
		return math.MaxUint32
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}
