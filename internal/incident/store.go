package incident

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const schema = `
CREATE TABLE IF NOT EXISTS incidents (
	id          INTEGER PRIMARY KEY,
	start_ms    INTEGER NOT NULL,
	end_ms      INTEGER,          -- NULL while ongoing
	updated_ms  INTEGER NOT NULL, -- last time the detector touched it
	location    INTEGER NOT NULL,
	baseline_ms REAL NOT NULL,
	peak_ms     REAL NOT NULL,
	lost        INTEGER NOT NULL,
	probes      INTEGER NOT NULL,
	suspects    TEXT NOT NULL,    -- JSON array of Suspect
	outage      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS incidents_start ON incidents (start_ms);
`

// Store keeps incidents in the hub's SQLite database.
type Store struct {
	db *sql.DB
}

// OpenStore creates the table if needed. An incident still marked ongoing
// was cut short by a hub restart; it's closed at its last update.
func OpenStore(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("create incidents table: %w", err)
	}
	// Databases from before outages existed lack the column.
	if err := addColumnIfMissing(db, "incidents", "outage", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return nil, err
	}
	if _, err := db.Exec(`UPDATE incidents SET end_ms = updated_ms WHERE end_ms IS NULL`); err != nil {
		return nil, fmt.Errorf("close interrupted incidents: %w", err)
	}
	return &Store{db: db}, nil
}

// Save inserts a new incident (setting its ID) or updates an existing one.
func (s *Store) Save(inc *Incident, now time.Time) error {
	suspects, err := json.Marshal(inc.Suspects)
	if err != nil {
		return err
	}
	var end any
	if !inc.End.IsZero() {
		end = inc.End.UnixMilli()
	}
	args := []any{inc.Start.UnixMilli(), end, now.UnixMilli(), inc.Location, inc.BaselineMS, inc.PeakMS, inc.Lost, inc.Probes, suspects, inc.Outage}

	if inc.ID == 0 {
		res, err := s.db.Exec(`INSERT INTO incidents
			(start_ms, end_ms, updated_ms, location, baseline_ms, peak_ms, lost, probes, suspects, outage)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, args...)
		if err != nil {
			return fmt.Errorf("insert incident: %w", err)
		}
		inc.ID, err = res.LastInsertId()
		return err
	}
	_, err = s.db.Exec(`UPDATE incidents SET
		start_ms = ?, end_ms = ?, updated_ms = ?, location = ?, baseline_ms = ?,
		peak_ms = ?, lost = ?, probes = ?, suspects = ?, outage = ?
		WHERE id = ?`, append(args, inc.ID)...)
	if err != nil {
		return fmt.Errorf("update incident: %w", err)
	}
	return nil
}

// List returns incidents still going or that ended at or after since,
// newest first.
func (s *Store) List(since time.Time) ([]Incident, error) {
	rows, err := s.db.Query(`SELECT id, start_ms, end_ms, location, baseline_ms, peak_ms, lost, probes, suspects, outage
		FROM incidents WHERE end_ms IS NULL OR end_ms >= ?
		ORDER BY start_ms DESC`, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var out []Incident
	for rows.Next() {
		var inc Incident
		var start int64
		var end sql.NullInt64
		var suspects string
		if err := rows.Scan(&inc.ID, &start, &end, &inc.Location, &inc.BaselineMS, &inc.PeakMS, &inc.Lost, &inc.Probes, &suspects, &inc.Outage); err != nil {
			return nil, err
		}
		inc.Start = time.UnixMilli(start)
		if end.Valid {
			inc.End = time.UnixMilli(end.Int64)
		}
		if err := json.Unmarshal([]byte(suspects), &inc.Suspects); err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}

// Prune deletes incidents that ended before the cutoff. Ongoing incidents
// are never deleted.
func (s *Store) Prune(before time.Time) error {
	if _, err := s.db.Exec(`DELETE FROM incidents WHERE end_ms IS NOT NULL AND end_ms < ?`, before.UnixMilli()); err != nil {
		return fmt.Errorf("prune incidents: %w", err)
	}
	return nil
}

// addColumnIfMissing upgrades a table created by an older version.
func addColumnIfMissing(db *sql.DB, table, column, def string) error {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, def)); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}
