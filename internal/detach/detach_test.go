package detach

import (
	"slices"
	"testing"
)

func TestWithoutDetach(t *testing.T) {
	got := withoutDetach([]string{"-hub", "http://x:8080", "--detach", "-detach=true", "-db", "a.db", "-detach=1"})
	want := []string{"-hub", "http://x:8080", "-db", "a.db"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// -detach=false means "don't", so it stays (and does nothing).
	if got := withoutDetach([]string{"-detach=false"}); len(got) != 1 {
		t.Errorf("-detach=false was dropped: %v", got)
	}
}
