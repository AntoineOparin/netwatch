package probe

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// errTimeout means no matching reply arrived in time: a lost packet.
var errTimeout = errors.New("timeout")

// pinger sends ICMP echo requests ("pings") over one socket.
//
// Unprivileged mode uses a datagram ICMP socket ("udp4"), which macOS allows
// for every user and Linux allows when net.ipv4.ping_group_range includes the
// user. Privileged mode uses a raw socket ("ip4:icmp") and needs root or
// CAP_NET_RAW.
type pinger struct {
	conn       *icmp.PacketConn
	privileged bool
	id         int
	buf        []byte
}

func newPinger(privileged bool) (*pinger, error) {
	network := "udp4"
	if privileged {
		network = "ip4:icmp"
	}
	conn, err := icmp.ListenPacket(network, "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("open ICMP socket (%s): %w", network, err)
	}
	return &pinger{
		conn:       conn,
		privileged: privileged,
		id:         os.Getpid() & 0xffff,
		buf:        make([]byte, 1500),
	}, nil
}

func (p *pinger) Close() error { return p.conn.Close() }

// reply is what came back for a probe: an echo reply from the target, or a
// "time exceeded" from a router along the way when the TTL ran out.
type reply struct {
	from net.IP
	rtt  time.Duration
	// timeExceeded is true when a router dropped the packet because its TTL
	// hit zero. That router's address is how traceroute finds each hop.
	timeExceeded bool
}

// send sends one echo request to dst with the given TTL and sequence number,
// then waits up to timeout for the matching reply.
func (p *pinger) send(dst net.IP, ttl, seq int, timeout time.Duration) (reply, error) {
	if err := p.conn.IPv4PacketConn().SetTTL(ttl); err != nil {
		return reply{}, fmt.Errorf("set TTL: %w", err)
	}
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Body: &icmp.Echo{ID: p.id, Seq: seq, Data: []byte("netwatch")},
	}
	b, err := msg.Marshal(nil)
	if err != nil {
		return reply{}, err
	}

	start := time.Now()
	deadline := start.Add(timeout)
	if _, err := p.conn.WriteTo(b, p.addr(dst)); err != nil {
		return reply{}, fmt.Errorf("send: %w", err)
	}
	if err := p.conn.SetReadDeadline(deadline); err != nil {
		return reply{}, err
	}

	// The socket can also see replies meant for other probes (late replies
	// to an earlier seq, or other sockets' traffic on macOS), so keep reading
	// until ours arrives or the deadline passes.
	for {
		n, peer, err := p.conn.ReadFrom(p.buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return reply{}, errTimeout
			}
			return reply{}, fmt.Errorf("receive: %w", err)
		}
		rtt := time.Since(start)
		from := peerIP(peer)

		m, err := icmp.ParseMessage(1, p.buf[:n]) // 1 = ICMP for IPv4
		if err != nil {
			continue
		}
		switch body := m.Body.(type) {
		case *icmp.Echo:
			// The kernel rewrites the ID on Linux datagram sockets, so only
			// the sender address and sequence number are reliable here.
			if m.Type == ipv4.ICMPTypeEchoReply && from.Equal(dst) && body.Seq == seq {
				return reply{from: from, rtt: rtt}, nil
			}
		case *icmp.TimeExceeded:
			if s, ok := quotedEchoSeq(body.Data); ok && s == seq {
				return reply{from: from, rtt: rtt, timeExceeded: true}, nil
			}
		}
	}
}

func (p *pinger) addr(ip net.IP) net.Addr {
	if p.privileged {
		return &net.IPAddr{IP: ip}
	}
	return &net.UDPAddr{IP: ip}
}

func peerIP(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.IP
	case *net.IPAddr:
		return a.IP
	}
	return nil
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
