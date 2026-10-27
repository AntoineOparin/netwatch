// Package hub is the central process: agents report to it, it stores what
// they send, and it serves the dashboard.
package hub

import (
	"context"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/gen/netwatch/v1/netwatchv1connect"
	"github.com/AntoineOparin/netwatch/internal/series"
)

// An agent counts as offline after missing this many heartbeats.
const missedHeartbeats = 3

type Server struct {
	registry          *Registry
	store             *series.Store
	heartbeatInterval time.Duration
}

func NewServer(registry *Registry, store *series.Store, heartbeatInterval time.Duration) *Server {
	return &Server{registry: registry, store: store, heartbeatInterval: heartbeatInterval}
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
	s.registry.Upsert(AgentInfo{
		ID:           msg.GetAgentId(),
		Hostname:     msg.GetHostname(),
		OS:           msg.GetOs(),
		Arch:         msg.GetArch(),
		AgentVersion: msg.GetAgentVersion(),
		IPs:          msg.GetIps(),
	})
	return connect.NewResponse(&netwatchv1.RegisterResponse{
		HeartbeatInterval: durationpb.New(s.heartbeatInterval),
	}), nil
}

func (s *Server) Heartbeat(
	_ context.Context,
	req *connect.Request[netwatchv1.HeartbeatRequest],
) (*connect.Response[netwatchv1.HeartbeatResponse], error) {
	if !s.registry.Touch(req.Msg.GetAgentId()) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("unknown agent, register again"))
	}
	return connect.NewResponse(&netwatchv1.HeartbeatResponse{}), nil
}

func (s *Server) ListAgents(
	context.Context,
	*connect.Request[netwatchv1.ListAgentsRequest],
) (*connect.Response[netwatchv1.ListAgentsResponse], error) {
	window := missedHeartbeats * s.heartbeatInterval
	infos := s.registry.List()
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
			Online:       s.registry.Online(info, window),
		})
	}
	return connect.NewResponse(&netwatchv1.ListAgentsResponse{Agents: agents}), nil
}
