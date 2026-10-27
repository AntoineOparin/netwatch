package hub

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/AntoineOparin/netwatch/internal/series"
)

const defaultHistory = 5 * time.Minute

// watch streams the given metrics to a dashboard client: everything within
// history first, in time order, then each new sample as it is stored.
//
// It subscribes before reading history, so a sample stored in between shows
// up on the subscription; anything already covered by history is skipped,
// so nothing is missed or sent twice.
//
// toPoint converts a stored value to the RPC's point type, or returns nil to
// leave it out. send delivers one batch.
func watch[P any](
	ctx context.Context,
	store *series.Store,
	metrics []string,
	history time.Duration,
	toPoint func(device, metric string, labels map[string]string, at time.Time, value float64) *P,
	send func([]*P) error,
) error {
	if history <= 0 {
		history = defaultHistory
	}
	live, unsubscribe := store.Subscribe(256)
	defer unsubscribe()

	type timed struct {
		at time.Time
		p  *P
	}
	var batch []timed
	lastSent := map[string]time.Time{}
	for _, metric := range metrics {
		all, err := store.Query(metric, time.Now().Add(-history))
		if err != nil {
			return err
		}
		for _, ser := range all {
			for _, pt := range ser.Points {
				if p := toPoint(ser.Device, metric, ser.Labels, pt.At, pt.Value); p != nil {
					batch = append(batch, timed{pt.At, p})
				}
			}
			if n := len(ser.Points); n > 0 {
				lastSent[seriesKey(ser.Device, metric, ser.Labels)] = ser.Points[n-1].At
			}
		}
	}
	slices.SortStableFunc(batch, func(a, b timed) int { return a.at.Compare(b.at) })
	points := make([]*P, len(batch))
	for i, t := range batch {
		points[i] = t.p
	}
	if err := send(points); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case smp := <-live:
			if !slices.Contains(metrics, smp.Metric) {
				continue
			}
			key := seriesKey(smp.Device, smp.Metric, smp.Labels)
			if !smp.At.After(lastSent[key]) {
				continue
			}
			lastSent[key] = smp.At
			if p := toPoint(smp.Device, smp.Metric, smp.Labels, smp.At, smp.Value); p != nil {
				if err := send([]*P{p}); err != nil {
					return err
				}
			}
		}
	}
}

func seriesKey(device, metric string, labels map[string]string) string {
	var b strings.Builder
	b.WriteString(device + "|" + metric)
	for _, k := range slices.Sorted(maps.Keys(labels)) {
		b.WriteString("|" + k + "=" + labels[k])
	}
	return b.String()
}
