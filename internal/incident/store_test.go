package incident

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestStoreSaveListAndRecover(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := OpenStore(db)
	if err != nil {
		t.Fatal(err)
	}

	inc := &Incident{Start: t0, Location: LocationUpstream, BaselineMS: 20, PeakMS: 90}
	if err := s.Save(inc, t0); err != nil {
		t.Fatal(err)
	}
	if inc.ID == 0 {
		t.Fatal("Save did not assign an ID")
	}
	inc.PeakMS = 180
	inc.Suspects = []Suspect{{Device: "dads-laptop", PeakUpBps: 1_125_000}}
	if err := s.Save(inc, t0.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}

	got, err := s.List(t0.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PeakMS != 180 || !got[0].End.IsZero() || got[0].Suspects[0].Device != "dads-laptop" {
		t.Fatalf("List() = %+v, want the updated ongoing incident", got)
	}

	// The hub restarts mid-incident: reopening closes it at its last update.
	if s, err = OpenStore(db); err != nil {
		t.Fatal(err)
	}
	got, err = s.List(t0.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if want := t0.Add(10 * time.Second); !got[0].End.Equal(want) {
		t.Fatalf("End = %v, want %v", got[0].End, want)
	}
}

func TestStoreUpgradesOldSchema(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// The table as M4 created it, before outages.
	if _, err := db.Exec(`CREATE TABLE incidents (id INTEGER PRIMARY KEY, start_ms INTEGER NOT NULL, end_ms INTEGER,
		updated_ms INTEGER NOT NULL, location INTEGER NOT NULL, baseline_ms REAL NOT NULL, peak_ms REAL NOT NULL,
		lost INTEGER NOT NULL, probes INTEGER NOT NULL, suspects TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	s, err := OpenStore(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(&Incident{Start: t0, End: t0.Add(time.Minute), Outage: true}, t0); err != nil {
		t.Fatal(err)
	}
	got, err := s.List(t0.Add(-time.Hour))
	if err != nil || len(got) != 1 || !got[0].Outage {
		t.Fatalf("got %+v, %v; want one outage", got, err)
	}
}

func TestStorePrune(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := OpenStore(db)
	if err != nil {
		t.Fatal(err)
	}
	old := &Incident{Start: t0, End: t0.Add(time.Minute)}
	recent := &Incident{Start: t0.Add(400 * 24 * time.Hour), End: t0.Add(400*24*time.Hour + time.Minute)}
	ongoing := &Incident{Start: t0} // started long ago, still open
	for _, inc := range []*Incident{old, recent, ongoing} {
		if err := s.Save(inc, inc.Start); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Prune(t0.Add(30 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	var ids []int64
	rows, err := db.Query(`SELECT id FROM incidents ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	if len(ids) != 2 || ids[0] != recent.ID || ids[1] != ongoing.ID {
		t.Fatalf("kept %v, want the recent and the ongoing incident", ids)
	}
}
