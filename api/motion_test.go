package api

import (
	"encoding/json"
	"testing"
	"time"
)

func rate(r float64) *float64 { return &r }

// obs builds an Observation without device.rate (wording-only fallback).
func obs(logID int64, text string, alert, pos int) Observation {
	return Observation{LogID: logID, Text: text, Alert: alert, Position: pos}
}

func TestClassifyActivity(t *testing.T) {
	cases := []struct {
		text string
		pos  int
		want string
	}{
		// Real messages from the SmartDoor cloud.
		{"Opening by Remote control", 0, "opening"},
		{"Closing by Remote control", 100, "closing"},
		{"Open by Remote control", 100, ""},   // finished opening
		{"Closed by Remote control", 0, ""},   // finished closing
		{"Daniel sent Open", 0, "opening"},    // app
		{"Daniel sent Close", 100, "closing"}, // app
		{"Closing by Home Assistant", 100, "closing"},
		// Synthetic.
		{"Opening by Daniel", 100, ""}, // already open
		{"Closing by Daniel", 0, ""},   // already closed
		{"Opened by Daniel", 100, ""},
		{"Stopped by Daniel", 40, "stopped"},
		{"Stopped by Daniel", 0, ""},
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

func TestIsAlertActivity(t *testing.T) {
	cases := []struct {
		text  string
		alert int
		want  bool
	}{
		// Real messages.
		{"Close Prevented as beam is blocked", 1, true},
		{"Opening by Safety Beam", 0, true}, // alert flag not set: wording needed
		{"Reversing", 1, true},
		{"Open after Obstruction", 1, true},
		{"Opening by Remote control", 0, false},
		{"Open by Remote control", 0, false},
		{"Closing by Remote control", 0, false},
		{"Closed by Remote control", 0, false},
		{"Daniel sent Open", 0, false},
		{"Daniel sent Close", 0, false},
		{"Closing by Home Assistant", 0, false},
		// Wording without the flag.
		{"PE beam blocked", 0, true},
		{"P.E. triggered", 0, true},
		{"Obstruction detected - door reversed", 0, true},
		{"Motor overload", 0, true},
		// Former false positives of plain substring matching.
		{"Pete sent Open", 0, false},       // "pe "
		{"Jamie sent Close", 0, false},     // "jam"
		{"Blockhouse sent Open", 0, false}, // "block"
		{"Hope sent Close", 0, false},      // "pe "
	}
	for _, c := range cases {
		if got := IsAlertActivity(c.text, c.alert); got != c.want {
			t.Errorf("IsAlertActivity(%q, %d) = %v, want %v", c.text, c.alert, got, c.want)
		}
	}
}

func TestMotionTracker(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()

	// First observation is a baseline only.
	if a := tr.Observe("rear", obs(10, "Closed by Remote control", 0, 0), now); a.Direction != "" || a.IsNew {
		t.Fatalf("baseline: got %+v", a)
	}
	// Same log entry again (e.g. periodic poll): nothing.
	if a := tr.Observe("rear", obs(10, "Closed by Remote control", 0, 0), now); a.Direction != "" || a.IsNew {
		t.Fatalf("repeat: got %+v", a)
	}
	// Another door's poll doesn't affect this one.
	tr.Observe("front", obs(5, "Closed", 0, 0), now)

	if a := tr.Observe("rear", obs(11, "Opening by Remote control", 0, 0), now); a.Direction != "opening" || !a.IsNew {
		t.Fatalf("start: got %+v", a)
	}
	if m, _ := tr.Moving("rear", now.Add(5*time.Second)); m != "opening" {
		t.Fatalf("moving: got %q", m)
	}
	if m, _ := tr.Moving("front", now); m != "" {
		t.Fatalf("front should not be moving, got %q", m)
	}
	if a := tr.Observe("rear", obs(12, "Open by Remote control", 0, 100), now); a.Direction != "" || !a.IsNew {
		t.Fatalf("finish: got %+v", a)
	}
	if !tr.NeedsResync("rear", "open", now) {
		t.Fatal("expected resync once the door reached open")
	}
	tr.Clear("rear")
	if m, _ := tr.Moving("rear", now); m != "" || tr.Dirty("rear") {
		t.Fatalf("cleared: got %q dirty=%v", m, tr.Dirty("rear"))
	}
}

func TestAlertWhileClosingResyncs(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("rear", obs(1, "Daniel sent Open", 0, 100), now) // baseline: door open

	if a := tr.Observe("rear", obs(2, "Daniel sent Close", 0, 100), now); a.Direction != "closing" {
		t.Fatalf("expected closing, got %+v", a)
	}
	if tr.NeedsResync("rear", "open", now.Add(5*time.Second)) {
		t.Fatal("should not resync while travelling")
	}
	a := tr.Observe("rear", obs(3, "Open after Obstruction", 1, 100), now.Add(8*time.Second))
	if !a.IsAlert || a.Direction != "" {
		t.Fatalf("expected alert with no direction, got %+v", a)
	}
	if !tr.NeedsResync("rear", "open", now.Add(9*time.Second)) {
		t.Fatal("expected resync after alert")
	}
	tr.Clear("rear")
	if tr.NeedsResync("rear", "open", now.Add(10*time.Second)) {
		t.Fatal("no resync after clear")
	}
}

func TestBeamWhileClosedIsNotOpening(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("rear", obs(1, "Daniel sent Close", 0, 0), now)
	a := tr.Observe("rear", obs(2, "PE beam blocked", 0, 0), now)
	if a.Direction != "" || !a.IsAlert {
		t.Fatalf("beam while closed must not be opening, got %+v", a)
	}
	if tr.NeedsResync("rear", "closed", now) {
		t.Fatal("nothing was published, no resync needed")
	}
}

func TestRealSmartDoorSequence(t *testing.T) {
	// Messages captured from the SmartDoor cloud on 2026-11-18 (no rate).
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("front", obs(1, "Closed by Remote control", 0, 0), now) // baseline: closed

	steps := []struct {
		text      string
		alert     int
		pos       int
		direction string
		isAlert   bool
	}{
		{"Opening by Remote control", 0, 0, "opening", false},
		{"Open by Remote control", 0, 100, "", false},
		{"Close Prevented as beam is blocked", 1, 100, "", true},
		{"Closing by Remote control", 0, 100, "closing", false},
		{"Opening by Safety Beam", 0, 91, "opening", true},
		{"Closing by Remote control", 0, 100, "closing", false},
		{"Reversing", 1, 62, "opening", true},
		{"Open after Obstruction", 1, 100, "", true},
		{"Daniel sent Close", 0, 100, "closing", false},
		{"Closed by Remote control", 0, 0, "", false},
	}
	for i, s := range steps {
		a := tr.Observe("front", obs(int64(i+2), s.text, s.alert, s.pos), now)
		if a.Direction != s.direction || a.IsAlert != s.isAlert || !a.IsNew {
			t.Errorf("%q: got %+v, want direction %q alert %v", s.text, a, s.direction, s.isAlert)
		}
	}
}

func TestSafetyBeamReversalResyncsOnArrival(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("front", obs(1, "Open by Remote control", 0, 100), now) // baseline: open
	tr.Observe("front", obs(2, "Closing by Remote control", 0, 100), now)
	if a := tr.Observe("front", obs(3, "Opening by Safety Beam", 0, 91), now.Add(4*time.Second)); a.Direction != "opening" {
		t.Fatalf("expected opening, got %+v", a)
	}
	if !tr.NeedsResync("front", "open", now.Add(6*time.Second)) {
		t.Fatal("expected immediate resync when the door reaches open")
	}
}

func TestOpeningNotResyncedWhileStillClosed(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("rear", obs(1, "Closed by Remote control", 0, 0), now)
	tr.Observe("rear", obs(2, "Opening by Remote control", 0, 0), now)
	if tr.NeedsResync("rear", "closed", now.Add(10*time.Second)) {
		t.Fatal("must not resync to closed while opening within grace")
	}
	if !tr.NeedsResync("rear", "closed", now.Add(60*time.Second)) {
		t.Fatal("expected resync after grace")
	}
}

func TestRateDrivesDirection(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	base := Observation{LogID: 1, Text: "Closed by Remote control", Position: 0, Rate: rate(0)}
	if a := tr.Observe("d", base, now); a.Direction != "" {
		t.Fatalf("baseline at closed: got %+v", a)
	}
	// Rate goes positive before any log entry: opening straight away.
	o := base
	o.Rate = rate(5)
	if a := tr.Observe("d", o, now); a.Direction != "opening" {
		t.Fatalf("rate>0: got %+v", a)
	}
	// The log entry arrives afterwards: no duplicate publish.
	o.LogID, o.Text = 2, "Opening by Remote control"
	if a := tr.Observe("d", o, now); a.Direction != "" || !a.IsNew {
		t.Fatalf("log after rate: got %+v", a)
	}
	// Stopped part-way: report stopped immediately, no grace wait.
	o.Rate, o.Position = rate(0), 40
	if a := tr.Observe("d", o, now.Add(5*time.Second)); a.Direction != "stopped" {
		t.Fatalf("rate 0 part-way: got %+v", a)
	}
	// Repeated polls while stopped don't republish.
	if a := tr.Observe("d", o, now.Add(65*time.Second)); a.Direction != "" {
		t.Fatalf("still stopped: got %+v", a)
	}
	// Closes again.
	o.Rate = rate(-3)
	if a := tr.Observe("d", o, now.Add(70*time.Second)); a.Direction != "closing" {
		t.Fatalf("rate<0: got %+v", a)
	}
	// Arrives closed: rate 0 at an end position is not "stopped".
	o.Rate, o.Position = rate(0), 0
	if a := tr.Observe("d", o, now.Add(90*time.Second)); a.Direction != "" {
		t.Fatalf("arrived: got %+v", a)
	}
	if !tr.NeedsResync("d", "closed", now.Add(90*time.Second)) {
		t.Fatal("expected resync on arrival")
	}
}

func TestRateOverridesWording(t *testing.T) {
	// "Close Prevented as beam is blocked" with rate 0 at 100: no movement.
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("d", Observation{LogID: 1, Position: 100, Rate: rate(0)}, now)
	a := tr.Observe("d", Observation{LogID: 2, Text: "Close Prevented as beam is blocked", Alert: 1, Position: 100, Rate: rate(0)}, now)
	if !a.IsAlert || a.Direction != "" {
		t.Fatalf("got %+v", a)
	}
	// "Reversing" with rate>0: opening (from rate).
	a = tr.Observe("d", Observation{LogID: 3, Text: "Reversing", Alert: 1, Position: 62, Rate: rate(4)}, now)
	if !a.IsAlert || a.Direction != "opening" {
		t.Fatalf("got %+v", a)
	}
}

func TestRestartMidMovement(t *testing.T) {
	now := time.Now()
	// With rate, a door already moving when the bridge starts is reported.
	tr := NewMotionTracker()
	if a := tr.Observe("d", Observation{LogID: 9, Text: "Closing by Remote control", Position: 100, Rate: rate(-4)}, now); a.Direction != "closing" || a.IsNew {
		t.Fatalf("moving at start: got %+v", a)
	}
	// One stopped part-way is reported as stopped.
	tr = NewMotionTracker()
	if a := tr.Observe("d", Observation{LogID: 9, Position: 30, Rate: rate(0)}, now); a.Direction != "stopped" {
		t.Fatalf("stopped at start: got %+v", a)
	}
	// Without rate, the old log entry is not replayed.
	tr = NewMotionTracker()
	if a := tr.Observe("d", obs(9, "Opening by Remote control", 0, 0), now); a.Direction != "" || a.IsNew {
		t.Fatalf("no rate at start: got %+v", a)
	}
}

func TestFSMStale(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("d", obs(1, "Open by Remote control", 0, 100), now)

	// Home Assistant sends close; FSM goes to closing.
	tr.NoteFSM("d", "closing", now)
	if tr.FSMStale("d", now.Add(5*time.Second)) {
		t.Fatal("not stale while within grace")
	}
	// The beam blocks the close: follow the reported position (open).
	tr.Observe("d", obs(2, "Close Prevented as beam is blocked", 1, 100), now.Add(6*time.Second))
	if !tr.FSMStale("d", now.Add(6*time.Second)) {
		t.Fatal("expected stale after a safety alert")
	}

	// A new command resets the timer; an older alert doesn't count.
	tr.NoteFSM("d", "open", now.Add(7*time.Second))
	tr.NoteFSM("d", "opening", now.Add(8*time.Second))
	if tr.FSMStale("d", now.Add(10*time.Second)) {
		t.Fatal("old alert must not make a new command stale")
	}
	if !tr.FSMStale("d", now.Add(60*time.Second)) {
		t.Fatal("expected stale after grace")
	}
}

// Sequences recorded on a real door (front door, 2026-11-19 field test).
func TestFieldTestStopPartWay(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("front", Observation{LogID: 1, Text: "Closed", Position: 0, Rate: rate(0)}, now)
	if a := tr.Observe("front", Observation{LogID: 2, Text: "Opening by Remote control", Position: 0, Rate: rate(5.263)}, now); a.Direction != "opening" {
		t.Fatalf("opening: got %+v", a)
	}
	a := tr.Observe("front", Observation{LogID: 3, Text: "Stopped by Remote control", Position: 48, Rate: rate(0)}, now.Add(11*time.Second))
	if a.Direction != StateStopped {
		t.Fatalf("stopped: got %+v", a)
	}
	// HA must be told "open", not "stopped": on "stopped" it showed Closed at 48%.
	if got := PublishedState(a.Direction); got != "open" {
		t.Fatalf("published %q, want open", got)
	}
}

func TestAppMoveWithZeroRate(t *testing.T) {
	// App commands report rate 0 while the door travels.
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("front", Observation{LogID: 1, Text: "Stopped by Home Assistant", Position: 42, Rate: rate(0)}, now)
	if a := tr.Observe("front", Observation{LogID: 2, Text: "Daniel sent Close", Position: 42, Rate: rate(0)}, now); a.Direction != "closing" {
		t.Fatalf("app close from part-way: got %+v", a)
	}

	// Even right after a rate-confirmed movement, the new entry wins over rate 0.
	tr.Observe("front", Observation{LogID: 3, Text: "Opening by Remote control", Position: 42, Rate: rate(5)}, now)
	if a := tr.Observe("front", Observation{LogID: 4, Text: "Daniel sent Close", Position: 70, Rate: rate(0)}, now); a.Direction != "closing" {
		t.Fatalf("app close after remote open: got %+v", a)
	}
}

func TestPublishedState(t *testing.T) {
	for in, want := range map[string]string{"stopped": "open", "opening": "opening", "closing": "closing", "open": "open", "closed": "closed"} {
		if got := PublishedState(in); got != want {
			t.Errorf("PublishedState(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodeRealDeviceJSON(t *testing.T) {
	raw := `{"deviceId":"abc","name":"Front","device":{"position":62,"rate":4.5},
		"log":{"text":"Reversing","time":1763452800,"source":7,"logId":1234,"alert":1}}`
	var d DoorStatusDevice
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	if d.Device.Rate == nil || *d.Device.Rate != 4.5 || d.Log.Source != 7 || d.Log.Alert != 1 || d.Log.ID != 1234 {
		t.Fatalf("decoded %+v", d)
	}
	var noRate DoorStatusDevice
	if err := json.Unmarshal([]byte(`{"device":{"position":0}}`), &noRate); err != nil || noRate.Device.Rate != nil {
		t.Fatalf("missing rate should be nil, got %v %v", noRate.Device.Rate, err)
	}
}
