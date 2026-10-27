package procnet

import (
	"testing"
	"time"
)

func TestParseNettopLine(t *testing.T) {
	tests := map[string]entry{
		"curl.54588,4550119,520,":          {name: "curl", pid: 54588, in: 4550119, out: 520},
		"com.apple.WebKi.78883,1467339,0,": {name: "com.apple.WebKi", pid: 78883, in: 1467339},
		"2.1.311.79936,93713752,1406714,":  {name: "2.1.311", pid: 79936, in: 93713752, out: 1406714},
	}
	for line, want := range tests {
		if got, ok := parseNettopLine(line); !ok || got != want {
			t.Errorf("parseNettopLine(%q) = %+v, %v; want %+v", line, got, ok, want)
		}
	}
	for _, bad := range []string{",bytes_in,bytes_out,", "noPID,1,2,", "x.12,abc,2,"} {
		if _, ok := parseNettopLine(bad); ok {
			t.Errorf("parseNettopLine(%q) parsed, want rejected", bad)
		}
	}
}

func TestCleanLine(t *testing.T) {
	for in, want := range map[string]string{
		"^D\b\b,bytes_in,bytes_out,\r": ",bytes_in,bytes_out,",
		"curl.54588,4550119,520,\r":    "curl.54588,4550119,520,",
		"curl.54588,1,2,":              "curl.54588,1,2,",
	} {
		if got := cleanLine(in); got != want {
			t.Errorf("cleanLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAppName(t *testing.T) {
	for exe, want := range map[string]string{
		"/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Framework.framework/Helpers/Google Chrome Helper.app/Contents/MacOS/Google Chrome Helper": "Google Chrome",
		"/usr/bin/curl": "curl",
		"/System/Library/CoreServices/Software Update.app/Contents/Resources/softwareupdated": "Software Update",
	} {
		if got := appName(exe); got != want {
			t.Errorf("appName(%q) = %q, want %q", exe, got, want)
		}
	}
}

func TestSummarize(t *testing.T) {
	names := map[int]string{1: "Google Chrome", 2: "Google Chrome", 3: "OneDrive", 4: "mDNSResponder"}
	got := summarize([]entry{
		{pid: 1, in: 2_000_000, out: 10_000},
		{pid: 2, in: 1_000_000},     // same app as pid 1
		{pid: 3, out: 4_000_000},    // uploading
		{pid: 4, in: 500, out: 400}, // below the noise floor
	}, 2*time.Second, func(e entry) string { return names[e.pid] })

	if len(got) != 2 {
		t.Fatalf("got %+v, want Chrome and OneDrive", got)
	}
	// Busiest (in + out) first: OneDrive's 2 MB/s beats Chrome's 1.5.
	if got[0].app != "OneDrive" || got[0].tx != 2_000_000 {
		t.Errorf("first = %+v, want OneDrive at 2 MB/s up", got[0])
	}
	if got[1].app != "Google Chrome" || got[1].rx != 1_500_000 || got[1].tx != 5_000 {
		t.Errorf("second = %+v, want Chrome at 1.5 MB/s down", got[1])
	}
}
