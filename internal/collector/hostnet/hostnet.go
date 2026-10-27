// Package hostnet measures how fast this computer is downloading and
// uploading, on the interface that carries its internet traffic.
//
// The OS keeps running totals of bytes received and sent per network
// interface. Reading them once a second and subtracting gives bytes per
// second. Only the uplink (the interface the default route uses: Wi-Fi or
// Ethernet) counts; VPN tunnels and virtual interfaces would double count.
package hostnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/jackpal/gateway"
	psnet "github.com/shirou/gopsutil/v4/net"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

const (
	MetricRx = "net_rx_bytes_per_sec"
	MetricTx = "net_tx_bytes_per_sec"

	// Finding the uplink shells out to the OS route table on some
	// platforms, so do it occasionally rather than every second.
	uplinkRecheck = 30 * time.Second
)

type counters struct {
	rx, tx uint64
}

type Collector struct {
	interval time.Duration
	// Swappable for tests.
	uplink func() (string, error)
	read   func(iface string) (counters, error)
	now    func() time.Time
}

func New(interval time.Duration) *Collector {
	return &Collector{interval: interval, uplink: uplinkInterface, read: readCounters, now: time.Now}
}

func (c *Collector) Name() string { return "hostnet" }

func (c *Collector) Run(ctx context.Context, out chan<- collector.Sample) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	var (
		iface     string
		checkedAt time.Time
		prev      counters
		prevAt    time.Time
		havePrev  bool
	)
	for {
		now := c.now()
		if now.Sub(checkedAt) >= uplinkRecheck {
			name, err := c.uplink()
			if err != nil {
				return err
			}
			if name != iface {
				iface, havePrev = name, false // new interface: start counting afresh
			}
			checkedAt = now
		}

		cur, err := c.read(iface)
		if err != nil {
			return err
		}
		// Counters only go down when the interface resets (sleep, driver
		// reload); that interval has no meaningful rate, so skip it.
		if havePrev && cur.rx >= prev.rx && cur.tx >= prev.tx {
			secs := now.Sub(prevAt).Seconds()
			labels := map[string]string{"iface": iface}
			for _, smp := range []collector.Sample{
				{Source: c.Name(), Metric: MetricRx, Labels: labels, Value: float64(cur.rx-prev.rx) / secs, At: now},
				{Source: c.Name(), Metric: MetricTx, Labels: labels, Value: float64(cur.tx-prev.tx) / secs, At: now},
			} {
				select {
				case out <- smp:
				case <-ctx.Done():
					return nil
				}
			}
		}
		prev, prevAt, havePrev = cur, now, true

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}
	}
}

func readCounters(iface string) (counters, error) {
	all, err := psnet.IOCounters(true)
	if err != nil {
		return counters{}, fmt.Errorf("read interface counters: %w", err)
	}
	for _, s := range all {
		if s.Name == iface {
			return counters{rx: s.BytesRecv, tx: s.BytesSent}, nil
		}
	}
	return counters{}, fmt.Errorf("no counters for interface %q", iface)
}

// uplinkInterface names the interface the default route goes out of.
func uplinkInterface() (string, error) {
	ip, err := gateway.DiscoverInterface()
	if err != nil {
		return "", fmt.Errorf("find uplink (offline?): %w", err)
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, ifc := range ifaces {
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.Equal(ip) {
				return ifc.Name, nil
			}
		}
	}
	return "", errors.New("uplink address " + ip.String() + " is on no interface")
}
