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
