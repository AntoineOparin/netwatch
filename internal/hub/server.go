// Package hub is the central process: agents report to it, it stores what
// they send, and it serves the dashboard.
package hub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/gen/netwatch/v1/netwatchv1connect"
	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/devices"
	"github.com/AntoineOparin/netwatch/internal/incident"
	"github.com/AntoineOparin/netwatch/internal/series"
)

const (
	// An agent counts as offline after missing this many reports.
	missedReports = 3
	// Upper bound on one report: 10 minutes of buffered samples is ~1200.
	maxReportSamples = 10_000
	// Clock differences above this get logged; they usually mean a machine
	// isn't syncing time.
	skewWarning = 2 * time.Second
)

// Config is everything the hub's API needs.
type Config struct {
	Registry  *Registry
	Store     *series.Store
	Incidents *incident.Store
	Devices   *devices.Store
	// How often agents report; three missed reports mean offline.
	ReportInterval time.Duration
	// A LAN device counts as online if a sweep saw it this recently.
	DeviceOnlineWindow time.Duration
	Log                *slog.Logger
}

type Server struct {
	cfg Config
}

func NewServer(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// Handler mounts the Connect services and falls back to ui for every other
// path, so one port serves both the API and the dashboard.
func (s *Server) Handler(ui http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(netwatchv1connect.NewAgentServiceHandler(s))
	mux.Handle(netwatchv1connect.NewDashboardServiceHandler(s))
	mux.Handle("/", ui)
	return mux
}

func (s *Server) Register(
	_ context.Context,
	req *connect.Request[netwatchv1.RegisterRequest],
) (*connect.Response[netwatchv1.RegisterResponse], error) {
	msg := req.Msg
	if msg.GetAgentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent_id is required"))
	}
	s.cfg.Registry.Upsert(AgentInfo{
		ID:           msg.GetAgentId(),
		Hostname:     msg.GetHostname(),
		OS:           msg.GetOs(),
		Arch:         msg.GetArch(),
		AgentVersion: msg.GetAgentVersion(),
		IPs:          msg.GetIps(),
	})
	return connect.NewResponse(&netwatchv1.RegisterResponse{
		ReportInterval: durationpb.New(s.cfg.ReportInterval),
	}), nil
}

func (s *Server) Report(
	_ context.Context,
	req *connect.Request[netwatchv1.ReportRequest],
) (*connect.Response[netwatchv1.ReportResponse], error) {
	msg := req.Msg
	id := msg.GetAgentId()
	if !s.cfg.Registry.Touch(id) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("unknown agent, register again"))
	}
	if n := len(msg.GetSamples()); n > maxReportSamples {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%d samples in one report, max %d", n, maxReportSamples))
	}

	// Move agent timestamps onto the hub's clock. The network delay folded
	// into this is a few milliseconds on a LAN, far less than the drift
	// between two machines that aren't syncing time.
	var skew time.Duration
	if msg.GetSentAt() != nil {
		skew = time.Since(msg.GetSentAt().AsTime())
		if skew > skewWarning || skew < -skewWarning {
			s.cfg.Log.Warn("agent clock is off", "agent_id", id, "skew", skew.Round(time.Millisecond))
		}
	}
	for _, smp := range msg.GetSamples() {
		err := s.cfg.Store.Append(collector.Sample{
			Source: "agent",
			Device: id,
			Metric: smp.GetMetric(),
			Labels: smp.GetLabels(),
			Value:  smp.GetValue(),
			At:     smp.GetAt().AsTime().Add(skew),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&netwatchv1.ReportResponse{}), nil
}

func (s *Server) ListAgents(
	context.Context,
	*connect.Request[netwatchv1.ListAgentsRequest],
) (*connect.Response[netwatchv1.ListAgentsResponse], error) {
	window := missedReports * s.cfg.ReportInterval
	infos := s.cfg.Registry.List()
	agents := make([]*netwatchv1.Agent, 0, len(infos))
	for _, info := range infos {
		agents = append(agents, &netwatchv1.Agent{
			Id:           info.ID,
			Hostname:     info.Hostname,
			Os:           info.OS,
			Arch:         info.Arch,
			AgentVersion: info.AgentVersion,
			Ips:          info.IPs,
			RegisteredAt: timestamppb.New(info.RegisteredAt),
			LastSeen:     timestamppb.New(info.LastSeen),
			Online:       s.cfg.Registry.Online(info, window),
		})
	}
	return connect.NewResponse(&netwatchv1.ListAgentsResponse{Agents: agents}), nil
}
