package wifi

import (
	"errors"
	"strings"
	"testing"
)

func TestParseDarwin(t *testing.T) {
	// Trimmed from a real `system_profiler SPAirPortDataType -json` (macOS 15).
	data := []byte(`{"SPAirPortDataType":[{"spairport_airport_interfaces":[
		{"_name":"en0","spairport_status_information":"spairport_status_connected",
		 "spairport_current_network_information":{"_name":"<redacted>",
		   "spairport_network_channel":"153 (5GHz, 80MHz)","spairport_network_rate":520,
		   "spairport_signal_noise":"-68 dBm / -95 dBm"}},
		{"_name":"awdl0","spairport_current_network_information":{"spairport_network_type":"x"}}
	]}]}`)
	r, err := parseDarwin(data)
	if err != nil {
		t.Fatal(err)
	}
	want := reading{signalDBm: -68, noiseDBm: -95, txRateMbps: 520, bandGHz: 5, channel: 153}
	if r != want {
		t.Fatalf("got %+v, want %+v", r, want)
	}

	if _, err := parseDarwin([]byte(`{"SPAirPortDataType":[{"spairport_airport_interfaces":[{"_name":"en0"}]}]}`)); !errors.Is(err, errNotConnected) {
		t.Errorf("Wi-Fi off: err = %v, want errNotConnected", err)
	}
}

func TestParseWindows(t *testing.T) {
	english := `
There is 1 interface on the system:

    Name                   : Wi-Fi
    State                  : connected
    Radio type             : 802.11ax
    Band                   : 5 GHz
    Channel                : 44
    Receive rate (Mbps)    : 1201
    Transmit rate (Mbps)   : 960.5
    Signal                 : 88%
`
	french := `
    Nom                    : Wi-Fi
    État                   : connecté
    Bande                  : 2,4 GHz
    Canal                  : 6
    Réception (Mbits/s)    : 144,4
    Transmission (Mbits/s) : 130
    Signal                 : 60 %
`
	tests := map[string]struct {
		out  string
		want reading
	}{
		"english": {english, reading{signalDBm: -56, txRateMbps: 960.5, bandGHz: 5, channel: 44}},
		"french":  {french, reading{signalDBm: -70, txRateMbps: 130, bandGHz: 2.4, channel: 6}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r, err := parseWindows(tt.out)
			if err != nil || r != tt.want {
				t.Fatalf("got %+v, %v; want %+v", r, err, tt.want)
			}
		})
	}

	disconnected := "    Name : Wi-Fi\n    State : disconnected\n"
	if _, err := parseWindows(disconnected); !errors.Is(err, errNotConnected) {
		t.Errorf("disconnected: err = %v, want errNotConnected", err)
	}
}

func TestParseLinux(t *testing.T) {
	out := `Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
 wlan0: 0000   54.  -56.  -256        0      0      0      0     12        0
`
	r, err := parseLinux(out)
	if err != nil || r != (reading{signalDBm: -56}) {
		t.Fatalf("got %+v, %v; want signal -56 and no noise", r, err)
	}
	if _, err := parseLinux(out[:strings.Index(out, " wlan0")]); !errors.Is(err, errNotConnected) {
		t.Errorf("no interfaces: err = %v, want errNotConnected", err)
	}
}
