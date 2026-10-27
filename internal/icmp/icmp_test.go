package icmp

import (
	"net"
	"testing"
	"time"
)

func TestQuotedEchoSeq(t *testing.T) {
	// 20-byte IPv4 header (IHL=5) followed by the start of an echo request
	// with ID 0x1234 and seq 0x0102.
	quoted := make([]byte, 28)
	quoted[0] = 0x45
	copy(quoted[20:], []byte{8, 0, 0xff, 0xff, 0x12, 0x34, 0x01, 0x02})

	seq, ok := quotedEchoSeq(quoted)
	if !ok || seq != 0x0102 {
		t.Fatalf("got (%d, %v), want (258, true)", seq, ok)
	}

	if _, ok := quotedEchoSeq(quoted[:24]); ok {
		t.Error("truncated quote should not parse")
	}
	quoted[20] = 0 // an echo reply, not our request
	if _, ok := quotedEchoSeq(quoted); ok {
		t.Error("non-echo-request quote should not parse")
	}
}

// TestPingLoopback sends a real ping to 127.0.0.1. It needs unprivileged
// ICMP (macOS, or Linux with ping_group_range), so it skips elsewhere.
func TestPingLoopback(t *testing.T) {
	pg, err := NewPinger(false)
	if err != nil {
		t.Skipf("unprivileged ICMP unavailable: %v", err)
	}
	defer pg.Close()

	r, err := pg.Send(net.IPv4(127, 0, 0, 1), 64, 7, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if r.TimeExceeded || !r.From.Equal(net.IPv4(127, 0, 0, 1)) || r.RTT <= 0 {
		t.Fatalf("unexpected reply %+v", r)
	}

	// 192.0.2.0/24 is reserved for documentation: nothing ever answers.
	got, err := pg.Sweep([]net.IP{net.IPv4(127, 0, 0, 1), net.IPv4(192, 0, 2, 1)}, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["127.0.0.1"]; !ok || len(got) != 1 {
		t.Fatalf("Sweep = %v, want only 127.0.0.1", got)
	}
}
