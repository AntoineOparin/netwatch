// Package icmp sends pings (ICMP echo requests) and reads the replies.
package icmp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	xicmp "golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ErrTimeout means no matching reply arrived in time: a lost packet.
var ErrTimeout = errors.New("timeout")

// Pinger sends pings over one socket. It is not safe for concurrent use.
//
// Unprivileged mode uses a datagram ICMP socket ("udp4"), which macOS allows
// for every user and Linux allows when net.ipv4.ping_group_range includes the
// user. Privileged mode uses a raw socket ("ip4:icmp") and needs root or
// CAP_NET_RAW.
type Pinger struct {
	conn       *xicmp.PacketConn
	privileged bool
	id         int
	buf        []byte
}

func NewPinger(privileged bool) (*Pinger, error) {
	network := "udp4"
	if privileged {
		network = "ip4:icmp"
	}
	conn, err := xicmp.ListenPacket(network, "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("open ICMP socket (%s): %w", network, err)
	}
	return &Pinger{
		conn:       conn,
		privileged: privileged,
		id:         os.Getpid() & 0xffff,
		buf:        make([]byte, 1500),
	}, nil
}

func (p *Pinger) Close() error { return p.conn.Close() }

// Reply is what came back for a ping: an echo reply from the target, or a
// "time exceeded" from a router along the way when the TTL ran out.
type Reply struct {
	From net.IP
	RTT  time.Duration
	// TimeExceeded is true when a router dropped the packet because its TTL
	// hit zero. That router's address is how traceroute finds each hop.
	TimeExceeded bool
}

// Send sends one echo request to dst with the given TTL and sequence number,
// then waits up to timeout for the matching reply.
func (p *Pinger) Send(dst net.IP, ttl, seq int, timeout time.Duration) (Reply, error) {
	if err := p.conn.IPv4PacketConn().SetTTL(ttl); err != nil {
		return Reply{}, fmt.Errorf("set TTL: %w", err)
	}
	start := time.Now()
	if err := p.write(dst, seq); err != nil {
		return Reply{}, err
	}
	if err := p.conn.SetReadDeadline(start.Add(timeout)); err != nil {
		return Reply{}, err
	}

	// The socket can also see replies meant for other pings (late replies
	// to an earlier seq, or other sockets' traffic on macOS), so keep reading
	// until ours arrives or the deadline passes.
	for {
		from, m, err := p.read()
		if err != nil {
			return Reply{}, err
		}
		rtt := time.Since(start)
		switch body := m.Body.(type) {
		case *xicmp.Echo:
			// The kernel rewrites the ID on Linux datagram sockets, so only
			// the sender address and sequence number are reliable here.
			if m.Type == ipv4.ICMPTypeEchoReply && from.Equal(dst) && body.Seq == seq {
				return Reply{From: from, RTT: rtt}, nil
			}
		case *xicmp.TimeExceeded:
			if s, ok := quotedEchoSeq(body.Data); ok && s == seq {
				return Reply{From: from, RTT: rtt, TimeExceeded: true}, nil
			}
		}
	}
}

// Sweep pings every address at once and returns the round trip of each one
// that answered within timeout, keyed by IP string. Hosts that don't answer
// are simply missing from the result.
func (p *Pinger) Sweep(ips []net.IP, timeout time.Duration) (map[string]time.Duration, error) {
	if err := p.conn.IPv4PacketConn().SetTTL(64); err != nil {
		return nil, fmt.Errorf("set TTL: %w", err)
	}
	sent := make(map[string]time.Time, len(ips))
	for i, ip := range ips {
		sent[ip.String()] = time.Now()
		if err := p.write(ip, i&0xffff); err != nil {
			// One unreachable address (e.g. no route) shouldn't stop the sweep.
			continue
		}
	}
	if err := p.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	got := map[string]time.Duration{}
	for len(got) < len(sent) {
		from, m, err := p.read()
		if errors.Is(err, ErrTimeout) {
			break
		}
		if err != nil {
			return nil, err
		}
		key := from.String()
		at, ok := sent[key]
		if _, dup := got[key]; ok && !dup && m.Type == ipv4.ICMPTypeEchoReply {
			got[key] = time.Since(at)
		}
	}
	return got, nil
}

func (p *Pinger) write(dst net.IP, seq int) error {
	msg := xicmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Body: &xicmp.Echo{ID: p.id, Seq: seq, Data: []byte("netwatch")},
	}
	b, err := msg.Marshal(nil)
	if err != nil {
		return err
	}
	var addr net.Addr = &net.UDPAddr{IP: dst}
	if p.privileged {
		addr = &net.IPAddr{IP: dst}
	}
	if _, err := p.conn.WriteTo(b, addr); err != nil {
		return fmt.Errorf("send to %s: %w", dst, err)
	}
	return nil
}

// read returns the next ICMP message and its sender, skipping anything that
// doesn't parse.
func (p *Pinger) read() (net.IP, *xicmp.Message, error) {
	for {
		n, peer, err := p.conn.ReadFrom(p.buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return nil, nil, ErrTimeout
			}
			return nil, nil, fmt.Errorf("receive: %w", err)
		}
		m, err := xicmp.ParseMessage(1, p.buf[:n]) // 1 = ICMP for IPv4
		if err != nil {
			continue
		}
		var from net.IP
		switch a := peer.(type) {
		case *net.UDPAddr:
			from = a.IP
		case *net.IPAddr:
			from = a.IP
		}
		return from, m, nil
	}
}

// quotedEchoSeq digs our sequence number out of a "time exceeded" message.
// Routers quote the start of the packet they dropped: its IP header (length
// in the low 4 bits of byte 0, in 32-bit words) followed by at least the
// first 8 bytes of our ICMP echo: type, code, checksum, ID, then seq.
func quotedEchoSeq(quoted []byte) (int, bool) {
	if len(quoted) < 1 {
		return 0, false
	}
	ihl := int(quoted[0]&0x0f) * 4
	if ihl < 20 || len(quoted) < ihl+8 {
		return 0, false
	}
	echo := quoted[ihl:]
	if echo[0] != byte(ipv4.ICMPTypeEcho) {
		return 0, false
	}
	return int(binary.BigEndian.Uint16(echo[6:8])), true
}
