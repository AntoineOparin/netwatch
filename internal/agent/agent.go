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
	"google.golang.org/protobuf/types/known/timestamppb"

	netwatchv1 "github.com/AntoineOparin/netwatch/gen/netwatch/v1"
	"github.com/AntoineOparin/netwatch/gen/netwatch/v1/netwatchv1connect"
	"github.com/AntoineOparin/netwatch/internal/collector"
)

const (
	minBackoff = time.Second
	maxBackoff = 30 * time.Second
	// Used if the hub doesn't send an interval.
	defaultReportInterval = 2 * time.Second
	// Samples kept while the hub is unreachable: about 10 minutes of
	// throughput (2 samples a second), so a hub restart loses nothing.
	maxBuffered = 1200
)

type Config struct {
	HubURL     string
	IDFile     string
	Version    string
	Logger     *slog.Logger
	Collectors []collector.Collector
	// HTTPClient defaults to http.DefaultClient.
	HTTPClient connect.HTTPClient
}

// Run registers with the hub, runs the collectors, and reports what they
// measure until ctx is cancelled. It never gives up on an unreachable hub:
// laptops sleep, hubs restart.
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
		buf:    &buffer{max: maxBuffered},
	}
	go collector.RunAll(ctx, a.log, a.buf.add, cfg.Collectors...)

	for {
		interval, err := a.register(ctx)
		if err != nil {
			return nil // only fails once ctx is cancelled
		}
		a.report(ctx, interval)
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
	buf    *buffer
}

// register retries with exponential backoff until it succeeds or ctx ends,
// and returns the report interval the hub asked for.
func (a *agent) register(ctx context.Context) (time.Duration, error) {
	backoff := minBackoff
	for {
		resp, err := a.client.Register(ctx, connect.NewRequest(a.info))
		if err == nil {
			interval := resp.Msg.GetReportInterval().AsDuration()
			if interval <= 0 {
				interval = defaultReportInterval
			}
			a.log.Info("registered with hub", "report_interval", interval)
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

// report sends buffered samples every interval, until ctx ends or the hub
// answers NOT_FOUND. A report counts as a heartbeat even when it's empty.
// When a report fails, its samples go back in the buffer for the next try.
func (a *agent) report(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		samples := a.buf.take()
		req := &netwatchv1.ReportRequest{
			AgentId: a.id,
			Samples: toProto(samples),
			SentAt:  timestamppb.Now(),
		}
		_, err := a.client.Report(ctx, connect.NewRequest(req))
		if err == nil {
			continue
		}
		a.buf.putBack(samples)
		switch {
		case connect.CodeOf(err) == connect.CodeNotFound:
			return
		case errors.Is(err, context.Canceled):
			return
		default:
			a.log.Warn("report failed", "err", err, "buffered", a.buf.len())
		}
	}
}

func toProto(samples []collector.Sample) []*netwatchv1.Sample {
	out := make([]*netwatchv1.Sample, len(samples))
	for i, s := range samples {
		out[i] = &netwatchv1.Sample{
			Metric: s.Metric,
			Labels: s.Labels,
			At:     timestamppb.New(s.At),
			Value:  s.Value,
		}
	}
	return out
}
