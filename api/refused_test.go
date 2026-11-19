package api

import (
	"testing"
	"time"
)

// Recorded on the front door, 20 Nov 06:39: Close from Home Assistant with the
// beam blocked while the door was at 83%. The hub logged only "Home Assistant
// sent Close" (rate 0) and pushed the beam alert to the app, not the log.
func TestRefusedCloseFromPartOpen(t *testing.T) {
	tr := NewMotionTracker()
	t0 := time.Now()
	tr.Observe("front", Observation{LogID: 1, Text: "Stopped by Home Assistant", Position: 83, Rate: rate(0)}, t0)
	tr.NoteFSM("front", "open", t0)

	if !tr.CommandSent("front", StateClosing, "Close", t0) {
		t.Fatal("command at 83% should be tracked")
	}
	tr.NoteFSM("front", "closing", t0)
	a := tr.Observe("front", Observation{LogID: 2, Text: "Home Assistant sent Close", Position: 83, Rate: rate(0)}, t0.Add(3*time.Second))
	if a.Direction != StateClosing {
		t.Fatalf("sent Close: got %+v", a)
	}
	if _, refused := tr.Refused("front", t0.Add(5*time.Second)); refused {
		t.Fatal("refused too early")
	}

	// Status fetched after RefusedAfter: nothing changed.
	at := t0.Add(RefusedAfter + time.Second)
	tr.Observe("front", Observation{LogID: 2, Text: "Home Assistant sent Close", Position: 83, Rate: rate(0)}, at)
	label, refused := tr.Refused("front", at)
	if !refused || label != "Close" {
		t.Fatalf("Refused = %q, %v; want Close, true", label, refused)
	}
	if moving, _ := tr.Moving("front", at); moving != "" {
		t.Fatalf("still moving %q after refusal", moving)
	}
	if !tr.FSMStale("front", at) {
		t.Fatal("FSM closing should give way to the reported position after a refusal")
	}
	if _, again := tr.Refused("front", at.Add(time.Second)); again {
		t.Fatal("refusal reported twice")
	}
}

// 20 Nov 06:34: a normal close from 78% reports a rate straight away.
func TestCloseThatMovesIsNotRefused(t *testing.T) {
	tr := NewMotionTracker()
	t0 := time.Now()
	tr.Observe("front", Observation{LogID: 1, Text: "Opening by Home Assistant", Position: 78, Rate: rate(0)}, t0)
	tr.CommandSent("front", StateClosing, "Close", t0)
	tr.Observe("front", Observation{LogID: 2, Text: "Closing by Home Assistant", Position: 78, Rate: rate(-5.882)}, t0.Add(4*time.Second))
	if _, refused := tr.Refused("front", t0.Add(RefusedAfter+time.Second)); refused {
		t.Fatal("a door reporting a closing rate was treated as refused")
	}
}

// 19 Nov 20:21: Ventilation from Home Assistant logged "sent Ventilation" with
// rate 0, but the door moved: the position changed a few seconds later.
func TestPresetWithZeroRateThatMovesIsNotRefused(t *testing.T) {
	tr := NewMotionTracker()
	t0 := time.Now()
	tr.Observe("rear", Observation{LogID: 1, Text: "Closed", Position: 0, Rate: rate(0)}, t0)
	tr.CommandSent("rear", StateOpening, "Partial open", t0)
	tr.Observe("rear", Observation{LogID: 2, Text: "Home Assistant sent Ventilation", Position: 0, Rate: rate(0)}, t0.Add(2*time.Second))
	tr.Observe("rear", Observation{LogID: 2, Text: "Home Assistant sent Ventilation", Position: 24, Rate: rate(0)}, t0.Add(6*time.Second))
	if _, refused := tr.Refused("rear", t0.Add(RefusedAfter+time.Second)); refused {
		t.Fatal("a preset that changed the position was treated as refused")
	}
}

// 20 Nov 06:54: from fully open the hub logs the alert itself, so the bridge
// must not raise a second one.
func TestRefusedCloseFromOpenUsesHubAlert(t *testing.T) {
	tr := NewMotionTracker()
	t0 := time.Now()
	tr.Observe("front", Observation{LogID: 1, Text: "Opening by Home Assistant", Position: 100, Rate: rate(0)}, t0)
	tr.CommandSent("front", StateClosing, "Close", t0)
	a := tr.Observe("front", Observation{LogID: 2, Text: "Close Prevented as beam is blocked", Alert: 1, Position: 100, Rate: rate(0)}, t0.Add(3*time.Second))
	if !a.IsAlert {
		t.Fatal("hub alert not recognised")
	}
	if _, refused := tr.Refused("front", t0.Add(RefusedAfter+time.Second)); refused {
		t.Fatal("refusal reported on top of the hub's own alert")
	}
}

func TestCommandToCurrentEndIsIgnored(t *testing.T) {
	tr := NewMotionTracker()
	t0 := time.Now()
	if tr.CommandSent("front", StateClosing, "Close", t0) {
		t.Fatal("tracked a command for a door never seen")
	}
	tr.Observe("front", Observation{LogID: 1, Text: "Closed", Position: 0, Rate: rate(0)}, t0)
	if tr.CommandSent("front", StateClosing, "Close", t0) {
		t.Fatal("tracked Close for a door that is already closed")
	}
	if !tr.CommandSent("front", StateOpening, "Open", t0) {
		t.Fatal("Open on a closed door should be tracked")
	}
	if pos, ok := tr.Position("front"); !ok || pos != 0 {
		t.Fatalf("Position = %d, %v", pos, ok)
	}
}
