// Package agent is the process that runs on each computer and reports to the
// hub.
package agent

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/gen/netwatch/v1/netwatchv1connect"
)

const (
	minBackoff = time.Second
	maxBackoff = 30 * time.Second
	// Used if the hub doesn't send an interval.
	defaultHeartbeat = 5 * time.Second
)

type Config struct {
	HubURL  string
	IDFile  string
	Version string
	Logger  *slog.Logger
	// HTTPClient defaults to http.DefaultClient.
	HTTPClient connect.HTTPClient
}

// Run registers with the hub and sends heartbeats until ctx is cancelled.
// It never gives up on an unreachable hub: laptops sleep, hubs restart.
func Run(ctx context.Context, cfg Config) error {
	id, err := loadOrCreateID(cfg.IDFile)
	if err != nil {
		return err
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	a := &agent{
		id:     id,
		log:    cfg.Logger.With("agent_id", id),
		client: netwatchv1connect.NewAgentServiceClient(httpClient, cfg.HubURL),
		info:   hostInfo(id, cfg.Version),
	}

	for {
		interval, err := a.register(ctx)
		if err != nil {
			return nil // only fails once ctx is cancelled
		}
		a.heartbeat(ctx, interval)
		if ctx.Err() != nil {
			return nil
		}
		a.log.Info("hub no longer knows this agent, registering again")
	}
}

type agent struct {
	id     string
	log    *slog.Logger
	client netwatchv1connect.AgentServiceClient
	info   *netwatchv1.RegisterRequest
}

// register retries with exponential backoff until it succeeds or ctx ends,
// and returns the heartbeat interval the hub asked for.
func (a *agent) register(ctx context.Context) (time.Duration, error) {
	backoff := minBackoff
	for {
		resp, err := a.client.Register(ctx, connect.NewRequest(a.info))
		if err == nil {
			interval := resp.Msg.GetHeartbeatInterval().AsDuration()
			if interval <= 0 {
				interval = defaultHeartbeat
			}
			a.log.Info("registered with hub", "heartbeat_interval", interval)
			return interval, nil
		}
		a.log.Warn("register failed, retrying", "err", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// heartbeat ticks until ctx ends or the hub answers NOT_FOUND. Other errors
// (hub down, network blip) are logged and the ticking continues.
func (a *agent) heartbeat(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	req := &netwatchv1.HeartbeatRequest{AgentId: a.id}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		_, err := a.client.Heartbeat(ctx, connect.NewRequest(req))
		switch {
		case err == nil:
		case connect.CodeOf(err) == connect.CodeNotFound:
			return
		case errors.Is(err, context.Canceled):
			return
		default:
			a.log.Warn("heartbeat failed", "err", err)
		}
	}
}
