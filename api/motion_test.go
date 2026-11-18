package api

import (
	"testing"
	"time"
)

func TestClassifyActivity(t *testing.T) {
	cases := []struct {
		text string
		pos  int
		want string
	}{
		{"Opening by Daniel", 0, "opening"},
		{"Closing by Daniel", 100, "closing"},
		{"Opening by Daniel", 100, ""}, // already open
		{"Closing by Daniel", 0, ""},   // already closed
		{"Opened by Daniel", 100, ""},  // finished movement
		{"Closed by Daniel", 0, ""},    // finished movement
		{"Operated by remote", 0, "opening"},
		{"Operated by remote", 100, "closing"},
		{"Operated by remote", 50, ""},
		{"", 0, ""},
	}
	for _, c := range cases {
		if got := ClassifyActivity(c.text, c.pos); got != c.want {
			t.Errorf("ClassifyActivity(%q, %d) = %q, want %q", c.text, c.pos, got, c.want)
		}
	}
}

func TestMotionTracker(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()

	// First observation is a baseline only.
	if dir, isNew := tr.Observe("rear", 10, "Closed by Daniel", 0, now); dir != "" || isNew {
		t.Fatalf("baseline: got %q %v", dir, isNew)
	}
	// Same log entry again (e.g. periodic poll): nothing.
	if dir, isNew := tr.Observe("rear", 10, "Closed by Daniel", 0, now); dir != "" || isNew {
		t.Fatalf("repeat: got %q %v", dir, isNew)
	}
	// Another door's poll doesn't affect this one.
	tr.Observe("front", 5, "Closed", 0, now)

	// New entry while closed: opening.
	dir, isNew := tr.Observe("rear", 11, "Opening by Daniel", 0, now)
	if dir != "opening" || !isNew {
		t.Fatalf("start: got %q %v", dir, isNew)
	}
	if m, _ := tr.Moving("rear", now.Add(5*time.Second)); m != "opening" {
		t.Fatalf("moving: got %q", m)
	}
	if m, _ := tr.Moving("front", now); m != "" {
		t.Fatalf("front should not be moving, got %q", m)
	}

	// Completion entry: new, but no direction.
	if dir, isNew := tr.Observe("rear", 12, "Opened by Daniel", 100, now); dir != "" || !isNew {
		t.Fatalf("finish: got %q %v", dir, isNew)
	}
	tr.Clear("rear")
	if m, _ := tr.Moving("rear", now); m != "" {
		t.Fatalf("cleared: got %q", m)
	}
}
