// Command agent runs on each computer and reports to the hub.
//
//	agent -hub http://192.168.0.42:8080
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AntoineOparin/netwatch/internal/agent"
	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/collector/hostnet"
	"github.com/AntoineOparin/netwatch/internal/collector/procnet"
)

// Set at build time: go build -ldflags "-X main.version=v0.1.0".
var version = "dev"

func main() {
	defaultIDFile, err := agent.DefaultIDFile()
	if err != nil {
		defaultIDFile = "agent-id"
	}
	hubURL := flag.String("hub", "http://localhost:8080", "hub base URL")
	idFile := flag.String("id-file", defaultIDFile, "where this agent's stable ID is stored")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("agent starting", "hub", *hubURL, "version", version)
	err = agent.Run(ctx, agent.Config{
		HubURL:     *hubURL,
		IDFile:     *idFile,
		Version:    version,
		Logger:     log,
		Collectors: collectors(),
	})
	if err != nil {
		log.Error("agent stopped", "err", err)
		os.Exit(1)
	}
}

func collectors() []collector.Collector {
	cs := []collector.Collector{hostnet.New(time.Second)}
	if procnet.Supported() {
		cs = append(cs, procnet.New())
	}
	return cs
}
