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
	"github.com/AntoineOparin/netwatch/internal/collector/discovery"
	"github.com/AntoineOparin/netwatch/internal/collector/probe"
	"github.com/AntoineOparin/netwatch/internal/devices"
	"github.com/AntoineOparin/netwatch/internal/hub"
	"github.com/AntoineOparin/netwatch/internal/incident"
	"github.com/AntoineOparin/netwatch/internal/series"
	"github.com/AntoineOparin/netwatch/internal/webui"
)

// Set at build time: go build -ldflags "-X main.version=v0.1.0".
var version = "dev"

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	reportInterval := flag.Duration("report-interval", 2*time.Second, "how often agents should report")
	dbPath := flag.String("db", "netwatch.db", "SQLite database file")
	probeInternet := flag.String("probe-internet", "1.1.1.1", "internet host to ping")
	probeGateway := flag.String("probe-gateway", "", "router IP to ping (default: detected)")
	probeISP := flag.String("probe-isp", "", `first ISP hop to ping (default: detected, "off" to skip)`)
	lagRise := flag.Float64("lag-rise", incident.DefaultConfig().MinRiseMS, "how many ms above normal internet ping counts as lag")
	keepHistory := flag.Duration("keep-history", series.DefaultHourRetention, "how long to keep hourly averages and lag incidents (8760h = 1 year)")
	forgetDevices := flag.Duration("forget-devices", 90*24*time.Hour, "forget devices not seen for this long (2160h = 90 days)")
	sweepEvery := flag.Duration("sweep-every", time.Minute, `how often to look for devices on the LAN ("0" turns discovery off)`)
	probePrivileged := flag.Bool("probe-privileged", false, "use raw ICMP sockets (needs root/CAP_NET_RAW; for Linux without ping_group_range)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := series.Open(*dbPath)
	if err != nil {
		log.Error("open database", "err", err)
		os.Exit(1)
	}
	defer store.Close()
	if err := store.SetHourRetention(*keepHistory); err != nil {
		log.Error("invalid -keep-history", "err", err)
		os.Exit(2)
	}
	go store.Maintain(ctx, log)

	incidents, err := incident.OpenStore(store.DB())
	if err != nil {
		log.Error("open incidents", "err", err)
		os.Exit(1)
	}
	devs, err := devices.OpenStore(store.DB())
	if err != nil {
		log.Error("open devices", "err", err)
		os.Exit(1)
	}

	go hub.RunRetention(ctx, hub.Retention{Incidents: *keepHistory, ForgetDevices: *forgetDevices}, incidents, devs, log)

	detectorConfig := incident.DefaultConfig()
	detectorConfig.MinRiseMS = *lagRise
	go hub.RunDetector(ctx, store, incident.NewDetector(detectorConfig), incidents, log)

	sink := func(smp collector.Sample) {
		if err := store.Append(smp); err != nil {
			log.Error("store sample", "err", err)
		}
	}
	collectors := []collector.Collector{
		probe.New(probe.Config{
			Internet:   *probeInternet,
			Gateway:    *probeGateway,
			ISP:        *probeISP,
			Interval:   probe.Interval,
			Timeout:    time.Second,
			Privileged: *probePrivileged,
			Logger:     log.With("collector", "probe"),
		}),
	}
	if *sweepEvery > 0 {
		collectors = append(collectors, discovery.New(discovery.Config{
			SweepEvery: *sweepEvery,
			PingEvery:  5 * time.Second,
			Privileged: *probePrivileged,
		}, devs))
	}
	go collector.RunAll(ctx, log, sink, collectors...)

	server := hub.NewServer(hub.Config{
		Registry:           hub.NewRegistry(),
		Store:              store,
		Incidents:          incidents,
		Devices:            devs,
		ReportInterval:     *reportInterval,
		DeviceOnlineWindow: 3 * *sweepEvery,
		Log:                log,
	})
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

	log.Info("hub listening", "addr", *addr, "db", *dbPath, "version", version)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("hub stopped", "err", err)
		os.Exit(1)
	}
}
