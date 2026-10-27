package devices

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/AntoineOparin/netwatch/internal/collector/discovery"
)

func TestStore(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := OpenStore(db)
	if err != nil {
		t.Fatal(err)
	}

	t0 := time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)
	xbox := discovery.Observation{MAC: "7c:ed:8d:01:02:03", IP: "192.168.0.50", Vendor: "Microsoft", Hostname: "XboxOne", Replied: true, At: t0}
	if err := s.Seen(xbox); err != nil {
		t.Fatal(err)
	}
	if err := s.SetNickname(xbox.MAC, "Xbox"); err != nil {
		t.Fatal(err)
	}

	// Later: new IP from DHCP, the router's DNS had no name this time, and
	// it didn't answer the ping (only ARP).
	if err := s.Seen(discovery.Observation{MAC: xbox.MAC, IP: "192.168.0.51", Vendor: "Microsoft", At: t0.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d devices, want 1", len(got))
	}
	d := got[0]
	switch {
	case d.IP != "192.168.0.51":
		t.Errorf("IP = %s, want the new one", d.IP)
	case d.Nickname != "Xbox" || d.Hostname != "XboxOne":
		t.Errorf("nickname %q hostname %q, want both kept", d.Nickname, d.Hostname)
	case !d.FirstSeen.Equal(t0) || !d.LastSeen.Equal(t0.Add(time.Hour)):
		t.Errorf("first %v last %v", d.FirstSeen, d.LastSeen)
	case !d.LastReply.Equal(t0):
		t.Errorf("LastReply = %v, want the earlier ping reply kept", d.LastReply)
	}

	if err := s.SetNickname("00:00:00:00:00:01", "ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetNickname on unknown device: err = %v, want ErrNotFound", err)
	}
}

func TestPrune(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := OpenStore(db)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)
	for _, o := range []discovery.Observation{
		{MAC: "aa:00:00:00:00:01", IP: "192.168.0.50", Hostname: "friends-phone", At: t0},
		{MAC: "aa:00:00:00:00:02", IP: "192.168.0.155", Self: true, At: t0},
		{MAC: "aa:00:00:00:00:03", IP: "192.168.0.177", Hostname: "XBOX", At: t0.Add(100 * 24 * time.Hour)},
	} {
		if err := s.Seen(o); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Prune(t0.Add(10 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Hostname != "XBOX" || !got[1].Self {
		t.Fatalf("kept %+v, want the Xbox and the hub's own machine", got)
	}
}
