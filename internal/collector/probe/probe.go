// Package probe measures latency and packet loss from the hub to three
// points on the path to the internet. Comparing them says where lag comes
// from:
//
//   - gateway  (your router):        bad here = Wi-Fi / LAN problem
//   - isp      (first hop past it):  bad here but gateway fine = your connection
//     is congested (someone saturating it) or the ISP's local node is struggling
//   - internet (e.g. 1.1.1.1):       bad only here = problem further out
package probe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"time"

	"github.com/jackpal/gateway"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

const (
	MetricRTT = "rtt_ms"

	TargetGateway  = "gateway"
	TargetISP      = "isp"
	TargetInternet = "internet"
)

type Config struct {
	// Internet is the far target. Required.
	Internet string
	// Gateway and ISP are detected when empty. ISP can be "off".
	Gateway string
	ISP     string
	// Interval between pings to each target; Timeout before a ping counts as lost.
	Interval   time.Duration
	Timeout    time.Duration
	Privileged bool
	Logger     *slog.Logger
}

// Probe pings each target every Interval. Each ping produces one rtt_ms
// sample; a lost ping is a NaN value, so loss and latency stay in one series.
type Probe struct {
	cfg Config
}

func New(cfg Config) *Probe { return &Probe{cfg: cfg} }

func (p *Probe) Name() string { return "probe" }

type target struct {
	name string
	ip   net.IP
}

func (p *Probe) Run(ctx context.Context, out chan<- collector.Sample) error {
	targets, err := p.resolveTargets()
	if err != nil {
		return err
	}
	// The first failure stops every target so RunAll can restart cleanly.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, len(targets))
	for _, t := range targets {
		p.cfg.Logger.Info("probing", "target", t.name, "addr", t.ip)
		go func() { errs <- p.ping(ctx, t, out) }()
	}
	err = <-errs
	cancel()
	for range len(targets) - 1 {
		<-errs
	}
	return err
}

// ping runs one target's loop on its own socket.
func (p *Probe) ping(ctx context.Context, t target, out chan<- collector.Sample) error {
	pg, err := newPinger(p.cfg.Privileged)
	if err != nil {
		return err
	}
	defer pg.Close()

	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	labels := map[string]string{"target": t.name, "addr": t.ip.String()}
	for seq := 0; ; seq = (seq + 1) & 0xffff {
		at := time.Now()
		value := math.NaN()
		r, err := pg.send(t.ip, 64, seq, p.cfg.Timeout)
		switch {
		case err == nil && !r.timeExceeded:
			value = float64(r.rtt.Microseconds()) / 1000
		case err == nil, errors.Is(err, errTimeout):
			// lost
		default:
			return fmt.Errorf("ping %s: %w", t.name, err)
		}

		select {
		case out <- collector.Sample{Source: p.Name(), Device: "hub", Metric: MetricRTT, Labels: labels, Value: value, At: at}:
		case <-ctx.Done():
			return nil
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func (p *Probe) resolveTargets() ([]target, error) {
	internet := net.ParseIP(p.cfg.Internet)
	if internet == nil {
		return nil, fmt.Errorf("internet target %q is not an IP address", p.cfg.Internet)
	}

	var gw net.IP
	if p.cfg.Gateway != "" {
		gw = net.ParseIP(p.cfg.Gateway)
	} else {
		var err error
		if gw, err = gateway.DiscoverGateway(); err != nil {
			return nil, fmt.Errorf("detect gateway (set it explicitly with -probe-gateway): %w", err)
		}
	}
	if gw == nil {
		return nil, fmt.Errorf("gateway %q is not an IP address", p.cfg.Gateway)
	}
	targets := []target{{TargetGateway, gw}}

	switch p.cfg.ISP {
	case "off":
	case "":
		isp, err := p.findISPHop(internet, gw)
		if err != nil {
			p.cfg.Logger.Warn("could not find the ISP hop, skipping it (set it with -probe-isp)", "err", err)
		} else {
			targets = append(targets, target{TargetISP, isp})
		}
	default:
		isp := net.ParseIP(p.cfg.ISP)
		if isp == nil {
			return nil, fmt.Errorf("ISP target %q is not an IP address", p.cfg.ISP)
		}
		targets = append(targets, target{TargetISP, isp})
	}

	return append(targets, target{TargetInternet, internet}), nil
}

// findISPHop finds the first router past the gateway, traceroute-style: send
// pings toward the internet target with TTL 2, 3, ... Each router decrements
// the TTL, and the one that drops it to zero answers "time exceeded", which
// reveals its address. Some routers stay silent, so try a few TTLs.
func (p *Probe) findISPHop(dst, gw net.IP) (net.IP, error) {
	pg, err := newPinger(p.cfg.Privileged)
	if err != nil {
		return nil, err
	}
	defer pg.Close()

	for ttl := 2; ttl <= 5; ttl++ {
		r, err := pg.send(dst, ttl, ttl, p.cfg.Timeout)
		if errors.Is(err, errTimeout) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if r.from.Equal(gw) {
			continue
		}
		if !r.timeExceeded {
			return nil, errors.New("reached the internet target directly; no hop in between")
		}
		return r.from, nil
	}
	return nil, errors.New("no router answered at TTL 2-5")
}
