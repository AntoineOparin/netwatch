package hub

import (
	"context"
	"log/slog"
	"time"

	"github.com/AntoineOparin/netwatch/internal/devices"
	"github.com/AntoineOparin/netwatch/internal/incident"
)

// Retention caps how long the hub keeps things that would otherwise grow
// forever. Time series have their own limits in the series package.
type Retention struct {
	Incidents     time.Duration // lag incidents and outages
	ForgetDevices time.Duration // devices not seen for this long
}

// RunRetention deletes expired incidents and forgotten devices once at
// startup and then every hour, until ctx is cancelled.
func RunRetention(ctx context.Context, r Retention, incidents *incident.Store, devs *devices.Store, log *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		now := time.Now()
		if err := incidents.Prune(now.Add(-r.Incidents)); err != nil {
			log.Error("retention", "err", err)
		}
		if err := devs.Prune(now.Add(-r.ForgetDevices)); err != nil {
			log.Error("retention", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
