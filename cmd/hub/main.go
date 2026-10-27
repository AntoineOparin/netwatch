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

	"github.com/AntoineOparin/netwatch/internal/hub"
	"github.com/AntoineOparin/netwatch/internal/webui"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	heartbeat := flag.Duration("heartbeat", 5*time.Second, "how often agents should check in")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := hub.NewServer(hub.NewRegistry(), *heartbeat)
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
