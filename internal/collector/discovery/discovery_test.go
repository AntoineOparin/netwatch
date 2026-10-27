package discovery

import (
	"net"
	"testing"
	"time"
)

var zeroTime time.Time

func TestHostsIn(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("192.168.0.155/24")
	hosts := hostsIn(subnet, net.ParseIP("192.168.0.155").To4())
	if len(hosts) != 253 { // 254 hosts minus self
		t.Fatalf("got %d hosts, want 253", len(hosts))
	}
	if hosts[0].String() != "192.168.0.1" || hosts[len(hosts)-1].String() != "192.168.0.254" {
		t.Errorf("range %s..%s, want .1...254", hosts[0], hosts[len(hosts)-1])
	}

	// A /16 is narrowed to the /24 around us.
	_, big, _ := net.ParseCIDR("10.1.2.3/16")
	if hosts := hostsIn(big, net.ParseIP("10.1.2.3").To4()); len(hosts) != 253 || hosts[0].String() != "10.1.2.1" {
		t.Errorf("/16: got %d hosts starting %s, want 253 from 10.1.2.1", len(hosts), hosts[0])
	}
}

func TestObserve(t *testing.T) {
	xbox, _ := net.ParseMAC("7c:ed:8d:01:02:03") // a Microsoft prefix
	o := observe(xbox, "192.168.0.50", "XboxOne", true, false, zeroTime)
	if o.Private || o.Vendor == "" {
		t.Errorf("real MAC: private=%v vendor=%q, want a vendor", o.Private, o.Vendor)
	}

	phone, _ := net.ParseMAC("ce:67:ae:01:02:03")
	if o := observe(phone, "192.168.0.167", "iPhone", false, false, zeroTime); !o.Private || o.Vendor != "" {
		t.Errorf("randomized MAC: private=%v vendor=%q, want private and no vendor", o.Private, o.Vendor)
	}
}

func TestShortName(t *testing.T) {
	for in, want := range map[string]string{
		"iPhone.hitronhub.home.": "iPhone",
		"puma7-atom.":            "puma7-atom",
		"":                       "",
	} {
		if got := shortName(in); got != want {
			t.Errorf("shortName(%q) = %q, want %q", in, got, want)
		}
	}
}
