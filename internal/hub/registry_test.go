package hub

import (
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	now := time.Date(2026, 10, 27, 21, 0, 0, 0, time.UTC)
	r := NewRegistry()
	r.now = func() time.Time { return now }

	if r.Touch("laptop") {
		t.Fatal("Touch on unknown agent should report false")
	}

	r.Upsert(AgentInfo{ID: "xbox-room-pc", Hostname: "zed"})
	r.Upsert(AgentInfo{ID: "dads-laptop", Hostname: "dad"})
	firstRegistered := now

	now = now.Add(10 * time.Second)
	if !r.Touch("dads-laptop") {
		t.Fatal("Touch on known agent should report true")
	}
	// A re-register keeps the original RegisteredAt.
	r.Upsert(AgentInfo{ID: "xbox-room-pc", Hostname: "zed"})

	got := r.List()
	if len(got) != 2 || got[0].Hostname != "dad" || got[1].Hostname != "zed" {
		t.Fatalf("List() = %+v, want dad then zed", got)
	}
	if !got[1].RegisteredAt.Equal(firstRegistered) {
		t.Errorf("RegisteredAt reset on re-register: got %v, want %v", got[1].RegisteredAt, firstRegistered)
	}

	now = now.Add(15 * time.Second)
	if !r.Online(got[0], 15*time.Second) {
		t.Error("agent seen exactly one window ago should be online")
	}
	now = now.Add(time.Nanosecond)
	if r.Online(got[0], 15*time.Second) {
		t.Error("agent seen more than one window ago should be offline")
	}
}
