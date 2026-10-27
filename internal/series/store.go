// Package series stores samples as time series in SQLite and fans new
// samples out to live subscribers (the dashboard's streams).
//
// Tables:
//
//	series      one row per (device, metric, labels)
//	samples     raw points, kept for RawRetention
//	samples_1m  per-minute rollups, kept for MinuteRetention
//	samples_1h  per-hour rollups, kept for HourRetention (1 year by default)
//
// NaN values (lost pings) are stored as NULL. SQLite's aggregate functions
// skip NULLs, so rollups average only real measurements and count the rest
// in n_missing.
package series

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo

	"github.com/AntoineOparin/netwatch/internal/collector"
)

const (
	RawRetention    = 24 * time.Hour
	MinuteRetention = 30 * 24 * time.Hour

	// DefaultHourRetention is how long per-hour rollups are kept unless
	// SetHourRetention says otherwise.
	DefaultHourRetention = 365 * 24 * time.Hour

	// Rollups are recomputed this far back each pass, so samples that arrive
	// late (an agent catching up after the hub was unreachable) are included.
	// Agents buffer at most 10 minutes.
	lateWindow = 15 * time.Minute
)

const schema = `
CREATE TABLE IF NOT EXISTS series (
	id     INTEGER PRIMARY KEY,
	device TEXT NOT NULL,
	metric TEXT NOT NULL,
	labels TEXT NOT NULL, -- JSON object, keys sorted
	UNIQUE (device, metric, labels)
);
CREATE TABLE IF NOT EXISTS samples (
	series_id INTEGER NOT NULL,
	ts        INTEGER NOT NULL, -- unix milliseconds
	value     REAL,             -- NULL = no value (lost ping)
	PRIMARY KEY (series_id, ts)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS samples_ts ON samples (ts);
`

// Both rollup tables share this shape; ts is the bucket start.
const rollupTable = `
CREATE TABLE IF NOT EXISTS %s (
	series_id INTEGER NOT NULL,
	ts        INTEGER NOT NULL,
	n         INTEGER NOT NULL, -- samples in the bucket
	n_missing INTEGER NOT NULL, -- of which NULL
	sum       REAL NOT NULL,
	min       REAL,
	max       REAL,
	PRIMARY KEY (series_id, ts)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS %[1]s_ts ON %[1]s (ts);
`

type Point struct {
	At    time.Time
	Value float64
}

// Series is every point for one (device, metric, labels) combination.
type Series struct {
	Device string
	Metric string
	Labels map[string]string
	Points []Point
}

type Store struct {
	db            *sql.DB
	now           func() time.Time
	hourRetention time.Duration

	mu     sync.Mutex
	ids    map[string]int64 // series key -> series.id
	subs   map[int]chan collector.Sample
	nextID int
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	// SQLite allows one writer at a time anyway, and the load here is a few
	// samples per second, so a single connection keeps things simple.
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{schema, fmt.Sprintf(rollupTable, "samples_1m"), fmt.Sprintf(rollupTable, "samples_1h")} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("create schema: %w", err)
		}
	}
	return &Store{
		db:            db,
		now:           time.Now,
		hourRetention: DefaultHourRetention,
		ids:           map[string]int64{},
		subs:          map[int]chan collector.Sample{},
	}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// SetHourRetention sets how long per-hour rollups are kept. It must be longer
// than MinuteRetention, or hours would vanish before the minutes they're
// built from. Call it before Maintain.
func (s *Store) SetHourRetention(d time.Duration) error {
	if d <= MinuteRetention {
		return fmt.Errorf("hourly history must be kept longer than %v", MinuteRetention)
	}
	s.hourRetention = d
	return nil
}

// DB exposes the database so sibling packages (e.g. incidents) can keep
// their own tables in the same file.
func (s *Store) DB() *sql.DB { return s.db }

// Append stores a sample and passes it to subscribers. Writing and
// publishing under one lock means a subscriber that then queries history
// sees every sample either in the history or on its channel. A subscriber
// that isn't keeping up misses samples rather than stalling every writer.
func (s *Store) Append(smp collector.Sample) error {
	// Stored timestamps have millisecond precision; publish the same value
	// so live samples and history compare equal.
	smp.At = time.UnixMilli(smp.At.UnixMilli())

	s.mu.Lock()
	defer s.mu.Unlock()

	id, err := s.seriesID(smp)
	if err != nil {
		return err
	}
	var value any = smp.Value
	if math.IsNaN(smp.Value) {
		value = nil
	}
	_, err = s.db.Exec(
		`INSERT INTO samples (series_id, ts, value) VALUES (?, ?, ?)
		 ON CONFLICT (series_id, ts) DO UPDATE SET value = excluded.value`,
		id, smp.At.UnixMilli(), value,
	)
	if err != nil {
		return fmt.Errorf("insert sample: %w", err)
	}

	for _, ch := range s.subs {
		select {
		case ch <- smp:
		default:
		}
	}
	return nil
}

// seriesID finds or creates the series row. Callers hold s.mu.
func (s *Store) seriesID(smp collector.Sample) (int64, error) {
	labels := smp.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labelJSON, err := json.Marshal(labels) // map keys are sorted
	if err != nil {
		return 0, err
	}
	key := smp.Device + "|" + smp.Metric + "|" + string(labelJSON)
	if id, ok := s.ids[key]; ok {
		return id, nil
	}
	if _, err := s.db.Exec(
		`INSERT INTO series (device, metric, labels) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
		smp.Device, smp.Metric, labelJSON,
	); err != nil {
		return 0, fmt.Errorf("insert series: %w", err)
	}
	var id int64
	if err := s.db.QueryRow(
		`SELECT id FROM series WHERE device = ? AND metric = ? AND labels = ?`,
		smp.Device, smp.Metric, labelJSON,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("find series: %w", err)
	}
	s.ids[key] = id
	return id, nil
}

// Query returns every series for metric with points at or after since. The
// further back since goes, the coarser the data: raw points within
// RawRetention, then per-minute averages, then per-hour averages.
func (s *Store) Query(metric string, since time.Time) ([]Series, error) {
	value, table := "p.value", s.table(since)
	if table != "samples" {
		value = rollupAvg
	}
	rows, err := s.db.Query(fmt.Sprintf(
		`SELECT s.device, s.labels, p.ts, %s
		 FROM %s p JOIN series s ON s.id = p.series_id
		 WHERE s.metric = ? AND p.ts >= ?
		 ORDER BY s.device, s.labels, p.ts`, value, table),
		metric, since.UnixMilli(),
	)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", metric, err)
	}
	defer rows.Close()

	var out []Series
	var lastKey string
	for rows.Next() {
		var device, labelJSON string
		var ts int64
		var v sql.NullFloat64
		if err := rows.Scan(&device, &labelJSON, &ts, &v); err != nil {
			return nil, err
		}
		if key := device + "|" + labelJSON; len(out) == 0 || key != lastKey {
			var labels map[string]string
			if err := json.Unmarshal([]byte(labelJSON), &labels); err != nil {
				return nil, err
			}
			out = append(out, Series{Device: device, Metric: metric, Labels: labels})
			lastKey = key
		}
		pt := Point{At: time.UnixMilli(ts), Value: math.NaN()}
		if v.Valid {
			pt.Value = v.Float64
		}
		cur := &out[len(out)-1]
		cur.Points = append(cur.Points, pt)
	}
	return out, rows.Err()
}

// CountPoints counts the samples (including NaN ones) of every series of
// metric that match, at or after since. Like Query, it reads raw samples
// within RawRetention and rollups beyond that.
func (s *Store) CountPoints(metric string, since time.Time, match func(device string, labels map[string]string) bool) (int64, error) {
	rows, err := s.db.Query(`SELECT id, device, labels FROM series WHERE metric = ?`, metric)
	if err != nil {
		return 0, fmt.Errorf("count %s: %w", metric, err)
	}
	var ids []any
	for rows.Next() {
		var id int64
		var device, labelJSON string
		if err := rows.Scan(&id, &device, &labelJSON); err != nil {
			rows.Close()
			return 0, err
		}
		var labels map[string]string
		if err := json.Unmarshal([]byte(labelJSON), &labels); err != nil {
			rows.Close()
			return 0, err
		}
		if match(device, labels) {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(ids) == 0 {
		return 0, err
	}

	count, table := "COUNT(*)", s.table(since)
	if table != "samples" {
		count = "COALESCE(SUM(n), 0)"
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	var n int64
	err = s.db.QueryRow(
		fmt.Sprintf(`SELECT %s FROM %s WHERE series_id IN (%s) AND ts >= ?`, count, table, placeholders),
		append(ids, since.UnixMilli())...,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count %s: %w", metric, err)
	}
	return n, nil
}

// table picks the finest table that still covers since. The slack keeps a
// "last 24 hours" query on raw samples: by the time it runs, since is a hair
// more than RawRetention ago.
func (s *Store) table(since time.Time) string {
	const slack = 5 * time.Minute
	switch age := s.now().Sub(since); {
	case age > MinuteRetention+slack:
		return "samples_1h"
	case age > RawRetention+slack:
		return "samples_1m"
	}
	return "samples"
}

// Average of the real measurements in a bucket; NULL if all were missing.
const rollupAvg = `CASE WHEN p.n > p.n_missing THEN p.sum / (p.n - p.n_missing) END`

// Subscribe returns a channel of every new sample. Call the returned func to
// unsubscribe; the channel is not closed, so stop reading at that point.
func (s *Store) Subscribe(buffer int) (<-chan collector.Sample, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID
	s.nextID++
	ch := make(chan collector.Sample, buffer)
	s.subs[id] = ch
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.subs, id)
	}
}

// Maintain rolls up and expires data once a minute until ctx is cancelled.
func (s *Store) Maintain(ctx context.Context, log *slog.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if err := s.maintain(); err != nil {
			log.Error("store maintenance failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// maintain folds finished minutes into samples_1m and finished hours into
// samples_1h, then deletes data past its retention.
func (s *Store) maintain() error {
	now := s.now()
	err := s.rollup("samples", "samples_1m", time.Minute, now,
		`COUNT(*), SUM(value IS NULL), TOTAL(value), MIN(value), MAX(value)`)
	if err != nil {
		return err
	}
	err = s.rollup("samples_1m", "samples_1h", time.Hour, now,
		`SUM(n), SUM(n_missing), TOTAL(sum), MIN(min), MAX(max)`)
	if err != nil {
		return err
	}
	for table, keep := range map[string]time.Duration{
		"samples":    RawRetention,
		"samples_1m": MinuteRetention,
		"samples_1h": s.hourRetention,
	} {
		if _, err := s.db.Exec(`DELETE FROM `+table+` WHERE ts < ?`, now.Add(-keep).UnixMilli()); err != nil {
			return fmt.Errorf("expire %s: %w", table, err)
		}
	}
	return s.pruneSeries()
}

// pruneSeries deletes series with no data left in any table: an app that
// was used once, a device that left. Holding s.mu keeps Append from writing
// to a series while it's being removed, and the ID cache is cleared so no
// stale ID is reused.
func (s *Store) pruneSeries() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM series WHERE
		NOT EXISTS (SELECT 1 FROM samples WHERE series_id = series.id) AND
		NOT EXISTS (SELECT 1 FROM samples_1m WHERE series_id = series.id) AND
		NOT EXISTS (SELECT 1 FROM samples_1h WHERE series_id = series.id)`)
	if err != nil {
		return fmt.Errorf("prune series: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		clear(s.ids)
	}
	return nil
}

// rollup aggregates src into dst buckets of the given size, for every bucket
// that has finished. It recomputes from a little before the newest bucket
// already in dst, so it is idempotent and catches late samples.
func (s *Store) rollup(src, dst string, bucket time.Duration, now time.Time, aggregates string) error {
	var newest sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(ts) FROM ` + dst).Scan(&newest); err != nil {
		return fmt.Errorf("rollup %s: %w", dst, err)
	}
	var from int64 // everything, the first time
	if newest.Valid {
		from = newest.Int64 - max(lateWindow, bucket).Milliseconds()
	}
	size := bucket.Milliseconds()
	_, err := s.db.Exec(fmt.Sprintf(
		`INSERT INTO %s (series_id, ts, n, n_missing, sum, min, max)
		 SELECT series_id, (ts / %d) * %[2]d AS bucket, %s
		 FROM %s WHERE ts >= ? AND ts < ?
		 GROUP BY series_id, bucket
		 ON CONFLICT (series_id, ts) DO UPDATE SET
		   n = excluded.n, n_missing = excluded.n_missing,
		   sum = excluded.sum, min = excluded.min, max = excluded.max`,
		dst, size, aggregates, src),
		from, now.Truncate(bucket).UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("rollup %s: %w", dst, err)
	}
	return nil
}
