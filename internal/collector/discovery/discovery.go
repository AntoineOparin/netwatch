// Package discovery finds every device on the local network, including
// ones that can't run an agent (consoles, phones, TVs), and measures the
// LAN round trip to those that answer pings.
//
// Every SweepEvery it pings each address in the subnet, which makes the OS
// resolve them with ARP, then reads the ARP table: anything listed is
// present, whether or not it answered the ping. Devices that do answer are
// pinged every PingEvery; that round trip shows how good their Wi-Fi is.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/endobit/oui"
	"github.com/jackpal/gateway"

	"github.com/AntoineOparin/netwatch/internal/collector"
	"github.com/AntoineOparin/netwatch/internal/icmp"
)

const (
	MetricLANRTT = "lan_rtt_ms"

	nameTTL     = 10 * time.Minute
	nameTimeout = time.Second
)

// Observation is one device seen during a sweep.
type Observation struct {
	MAC      string
	IP       string
	Vendor   string // maker, from the MAC; empty if unknown or private
	Private  bool   // randomized MAC: the maker can't be known
	Hostname string // from the router's DNS, if it has one
	Replied  bool   // answered the ping (not just ARP)
	Self     bool   // the machine running the hub
	Router   bool   // the default gateway
	At       time.Time
}

// Inventory receives every observation; the hub keeps the device list.
type Inventory interface {
	Seen(Observation) error
}

type Config struct {
	SweepEvery time.Duration
	PingEvery  time.Duration
	Privileged bool
}

type Collector struct {
	cfg Config
	inv Inventory

	mu       sync.Mutex
	names    map[string]cachedName // by IP
	resolver []*net.Resolver       // tried in order
}

type cachedName struct {
	name string
	at   time.Time
}

func New(cfg Config, inv Inventory) *Collector {
	return &Collector{cfg: cfg, inv: inv, names: map[string]cachedName{}}
}

func (c *Collector) Name() string { return "discovery" }

func (c *Collector) Run(ctx context.Context, out chan<- collector.Sample) error {
	self, err := localInterface()
	if err != nil {
		return err
	}
	pg, err := icmp.NewPinger(c.cfg.Privileged)
	if err != nil {
		return err
	}
	defer pg.Close()

	c.resolver = []*net.Resolver{net.DefaultResolver}
	var router string
	if gw, err := gateway.DiscoverGateway(); err == nil {
		router = gw.String()
		c.resolver = append([]*net.Resolver{routerResolver(gw)}, c.resolver...)
	}

	hosts := hostsIn(self.subnet, self.ip)
	responders := map[string]net.HardwareAddr{} // IP -> MAC of devices that answer pings
	emit := func(mac net.HardwareAddr, rtt time.Duration, ok bool, at time.Time) bool {
		value := math.NaN()
		if ok {
			value = float64(rtt.Microseconds()) / 1000
		}
		select {
		case out <- collector.Sample{Source: c.Name(), Device: mac.String(), Metric: MetricLANRTT, Value: value, At: at}:
			return true
		case <-ctx.Done():
			return false
		}
	}

	sweep := func() error {
		at := time.Now()
		replies, err := pg.Sweep(hosts, 2*time.Second)
		if err != nil {
			return err
		}
		table, err := readARP()
		if err != nil {
			return fmt.Errorf("read ARP table: %w", err)
		}
		for ip := range table {
			if !self.subnet.Contains(net.ParseIP(ip)) {
				delete(table, ip)
			}
		}
		names := c.lookupNames(append(keys(table), self.ip.String()))

		clear(responders)
		for ip, mac := range table {
			rtt, replied := replies[ip]
			o := observe(mac, ip, names[ip], replied, false, at)
			o.Router = ip == router
			if err := c.inv.Seen(o); err != nil {
				return err
			}
			if replied {
				responders[ip] = mac
				if !emit(mac, rtt, true, at) {
					return nil
				}
			}
		}
		// The hub's own machine never appears in its own ARP table.
		return c.inv.Seen(observe(self.mac, self.ip.String(), names[self.ip.String()], false, true, at))
	}

	if err := sweep(); err != nil {
		return err
	}
	sweepTicker := time.NewTicker(c.cfg.SweepEvery)
	defer sweepTicker.Stop()
	pingTicker := time.NewTicker(c.cfg.PingEvery)
	defer pingTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-sweepTicker.C:
			if err := sweep(); err != nil {
				return err
			}
		case <-pingTicker.C:
			at := time.Now()
			ips := make([]net.IP, 0, len(responders))
			for ip := range responders {
				ips = append(ips, net.ParseIP(ip))
			}
			replies, err := pg.Sweep(ips, time.Second)
			if err != nil {
				return err
			}
			for ip, mac := range responders {
				rtt, ok := replies[ip]
				if !emit(mac, rtt, ok, at) {
					return nil
				}
			}
		}
	}
}

func observe(mac net.HardwareAddr, ip, hostname string, replied, self bool, at time.Time) Observation {
	o := Observation{
		MAC:      mac.String(),
		IP:       ip,
		Private:  isPrivateMAC(mac),
		Hostname: hostname,
		Replied:  replied,
		Self:     self,
		At:       at,
	}
	if !o.Private {
		o.Vendor = oui.VendorFromMAC(mac)
	}
	return o
}

// lookupNames asks the router's DNS for each IP's name, in parallel, with
// answers cached for a while. Home routers usually know the name each
// device gave when it asked for its IP (DHCP), e.g. "iPhone.hitronhub.home".
func (c *Collector) lookupNames(ips []string) map[string]string {
	out := make(map[string]string, len(ips))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ip := range ips {
		c.mu.Lock()
		cached, ok := c.names[ip]
		c.mu.Unlock()
		if ok && time.Since(cached.at) < nameTTL {
			out[ip] = cached.name
			continue
		}
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), nameTimeout)
			defer cancel()
			var name string
			for _, r := range c.resolver {
				if names, err := r.LookupAddr(ctx, ip); err == nil && len(names) > 0 {
					name = shortName(names[0])
					break
				}
			}
			c.mu.Lock()
			c.names[ip] = cachedName{name, time.Now()}
			c.mu.Unlock()
			mu.Lock()
			out[ip] = name
			mu.Unlock()
		})
	}
	wg.Wait()
	return out
}

// routerResolver asks the router's own DNS server. The machine's configured
// DNS is often the ISP's (or a public one), which can't know LAN names; the
// router handed out the addresses, so it can.
func routerResolver(gw net.IP) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "udp", net.JoinHostPort(gw.String(), "53"))
		},
	}
}

// shortName keeps the first label: "iPhone.hitronhub.home." -> "iPhone".
func shortName(fqdn string) string {
	name, _, _ := strings.Cut(strings.TrimSuffix(fqdn, "."), ".")
	return name
}

type local struct {
	ip     net.IP
	subnet *net.IPNet
	mac    net.HardwareAddr
}

// localInterface finds the interface the default route uses, with its
// address and subnet.
func localInterface() (local, error) {
	ip, err := gateway.DiscoverInterface()
	if err != nil {
		return local{}, fmt.Errorf("find local network (offline?): %w", err)
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return local{}, err
	}
	for _, ifc := range ifaces {
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.Equal(ip) && ipnet.IP.To4() != nil {
				return local{ip: ip.To4(), subnet: ipnet, mac: ifc.HardwareAddr}, nil
			}
		}
	}
	return local{}, errors.New("no IPv4 interface has address " + ip.String())
}

// hostsIn lists every host address in the subnet except self. Subnets
// bigger than a /24 are narrowed to the /24 around self: a home network
// rarely needs more, and sweeping 65,000 addresses would be rude.
func hostsIn(subnet *net.IPNet, self net.IP) []net.IP {
	ones, bits := subnet.Mask.Size()
	if bits != 32 {
		return nil
	}
	if ones < 24 {
		subnet = &net.IPNet{IP: self.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}
		ones = 24
	}
	base := subnet.IP.Mask(subnet.Mask).To4()
	size := uint32(1) << (32 - ones)
	var out []net.IP
	for i := uint32(1); i < size-1; i++ { // skip network and broadcast addresses
		n := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
		n += i
		ip := net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n)).To4()
		if !ip.Equal(self) {
			out = append(out, ip)
		}
	}
	return out
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
