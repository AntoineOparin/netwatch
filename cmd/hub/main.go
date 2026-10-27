// Command hub collects reports from agents and serves the dashboard.
//
//	hub -addr :8080
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/collector/probe"
	"github.com/AntoineOparin/netwatch/internal/hub"
	"github.com/AntoineOparin/netwatch/internal/series"
	"github.com/AntoineOparin/netwatch/internal/webui"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	heartbeat := flag.Duration("heartbeat", 5*time.Second, "how often agents should check in")
	retention := flag.Duration("retention", 15*time.Minute, "how much history to keep in memory")
	probeInternet := flag.String("probe-internet", "1.1.1.1", "internet host to ping")
	probeGateway := flag.String("probe-gateway", "", "router IP to ping (default: detected)")
	probeISP := flag.String("probe-isp", "", `first ISP hop to ping (default: detected, "off" to skip)`)
	probePrivileged := flag.Bool("probe-privileged", false, "use raw ICMP sockets (needs root/CAP_NET_RAW; for Linux without ping_group_range)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := series.NewStore(*retention)
	go collector.RunAll(ctx, log, store.Append,
		probe.New(probe.Config{
			Internet:   *probeInternet,
			Gateway:    *probeGateway,
			ISP:        *probeISP,
			Interval:   time.Second,
			Timeout:    time.Second,
			Privileged: *probePrivileged,
			Logger:     log.With("collector", "probe"),
		}),
	)

	server := hub.NewServer(hub.NewRegistry(), store, *heartbeat)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           server.Handler(webui.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Info("hub listening", "addr", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("hub stopped", "err", err)
		os.Exit(1)
	}
}
