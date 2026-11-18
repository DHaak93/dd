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
	if a := tr.Observe("rear", 10, "Closed by Daniel", 0, 0, now); a.Direction != "" || a.IsNew {
		t.Fatalf("baseline: got %+v", a)
	}
	// Same log entry again (e.g. periodic poll): nothing.
	if a := tr.Observe("rear", 10, "Closed by Daniel", 0, 0, now); a.Direction != "" || a.IsNew {
		t.Fatalf("repeat: got %+v", a)
	}
	// Another door's poll doesn't affect this one.
	tr.Observe("front", 5, "Closed", 0, 0, now)

	// New entry while closed: opening.
	if a := tr.Observe("rear", 11, "Opening by Daniel", 0, 0, now); a.Direction != "opening" || !a.IsNew {
		t.Fatalf("start: got %+v", a)
	}
	if m, _ := tr.Moving("rear", now.Add(5*time.Second)); m != "opening" {
		t.Fatalf("moving: got %q", m)
	}
	if m, _ := tr.Moving("front", now); m != "" {
		t.Fatalf("front should not be moving, got %q", m)
	}

	// Completion entry: new, but no direction.
	if a := tr.Observe("rear", 12, "Opened by Daniel", 0, 100, now); a.Direction != "" || !a.IsNew {
		t.Fatalf("finish: got %+v", a)
	}
	tr.Clear("rear")
	if m, _ := tr.Moving("rear", now); m != "" {
		t.Fatalf("cleared: got %q", m)
	}
}

func TestAlertActivity(t *testing.T) {
	alerts := []string{"PE beam blocked", "Obstruction detected - door reversed", "P.E. Beam triggered", "Safety beam fault"}
	for _, a := range alerts {
		if !IsAlertActivity(a, 0) {
			t.Errorf("expected alert: %q", a)
		}
	}
	normal := []string{"Daniel sent Open", "Daniel sent Close", "Opened by remote", "Operated by wall button"}
	for _, n := range normal {
		if IsAlertActivity(n, 0) {
			t.Errorf("unexpected alert: %q", n)
		}
	}
}

func TestAlertWhileClosingResyncs(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("rear", 1, "Daniel sent Open", 0, 100, now) // baseline: door open

	// Close starts.
	if a := tr.Observe("rear", 2, "Daniel sent Close", 0, 100, now); a.Direction != "closing" {
		t.Fatalf("expected closing, got %+v", a)
	}
	// A poll while travelling must not resync.
	if tr.NeedsResync("rear", now.Add(5*time.Second)) {
		t.Fatal("should not resync while travelling")
	}
	// Obstruction reverses the door.
	a := tr.Observe("rear", 3, "Obstruction detected", 0, 100, now.Add(8*time.Second))
	if !a.IsAlert || a.Direction != "" {
		t.Fatalf("expected alert with no direction, got %+v", a)
	}
	// Door is back at 100 (the FSM still says open): re-announce open.
	if !tr.NeedsResync("rear", now.Add(20*time.Second)) {
		t.Fatal("expected resync after alert")
	}
	tr.Clear("rear")
	if tr.NeedsResync("rear", now.Add(21*time.Second)) {
		t.Fatal("no resync after clear")
	}
}

func TestBeamWhileClosedIsNotOpening(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("rear", 1, "Daniel sent Close", 0, 0, now)
	a := tr.Observe("rear", 2, "PE beam blocked", 0, 0, now)
	if a.Direction != "" || !a.IsAlert {
		t.Fatalf("beam while closed must not be opening, got %+v", a)
	}
	if tr.NeedsResync("rear", now) {
		t.Fatal("nothing was published, no resync needed")
	}
}
