//go:build darwin && cgo

package wifi

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation
#import <CoreWLAN/CoreWLAN.h>

typedef struct {
	int ok;
	long rssi;
	long noise;
	double txRate;
	long channel;
	int band; // CWChannelBand: 1 = 2.4 GHz, 2 = 5 GHz, 3 = 6 GHz
} wifiReading;

// readWifi asks CoreWLAN for the current link's numbers. These come from
// the driver's association state, so unlike `system_profiler` (which lists
// nearby networks) this never triggers a scan.
static wifiReading readWifi(void) {
	wifiReading r = {0};
	@autoreleasepool {
		CWInterface *ifc = [[CWWiFiClient sharedWiFiClient] interface];
		if (ifc == nil || !ifc.powerOn) return r;
		NSInteger rssi = ifc.rssiValue;
		if (rssi == 0) return r; // powered on but not associated
		r.ok = 1;
		r.rssi = rssi;
		r.noise = ifc.noiseMeasurement;
		r.txRate = ifc.transmitRate;
		CWChannel *ch = ifc.wlanChannel;
		if (ch != nil) {
			r.channel = ch.channelNumber;
			r.band = (int)ch.channelBand;
		}
	}
	return r;
}
*/
import "C"

import "context"

func read(context.Context) (reading, error) {
	r := C.readWifi()
	if r.ok == 0 {
		return reading{}, errNotConnected
	}
	return reading{
		signalDBm:  float64(r.rssi),
		noiseDBm:   float64(r.noise),
		txRateMbps: float64(r.txRate),
		bandGHz:    bandGHz(int(r.band)),
		channel:    int(r.channel),
	}, nil
}
