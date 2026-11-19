package api

import (
	"strings"
	"sync"
	"time"
)

// MotionGrace is how long a door is allowed to report "opening"/"closing"
// before a contradicting end position (or an intermediate position) is
// trusted again. It should be longer than the slowest door's travel time.
const MotionGrace = 45 * time.Second

// Door states published on the state topic that do not come from the FSM.
const (
	StateOpening = "opening"
	StateClosing = "closing"
	StateStopped = "stopped"
)

// PublishedState maps a direction from the tracker to the payload sent on the
// state topic. A door halted part-way is published as "open": Home
// Assistant's MQTT cover has no stopped state of its own, and on "stopped" it
// picks open or closed from the last position it received, which may still be
// the old end position (seen on a real door: "closed" at 48%).
func PublishedState(dir string) string {
	if dir == StateStopped {
		return "open"
	}
	return dir
}

// MotionTracker works out when a door starts or stops moving between end
// positions. The reported position is only updated at the end of travel, so
// without this Home Assistant would not see a door move until it had stopped.
//
// Signals, most reliable first:
//  1. device.rate: positive while opening, negative while closing, 0 when
//     stopped (only used when the cloud sends it).
//  2. log.alert: non-zero for safety events (beam blocked, obstruction,
//     reversing).
//  3. The wording of the cloud activity log entry ("Opening by Remote
//     control", "Daniel sent Close", "Reversing", ...).
//
// MotionTracker is safe for concurrent use.
type MotionTracker struct {
	mu      sync.Mutex
	devices map[string]*motionInfo
}

type motionInfo struct {
	logID  int64
	moving string    // "", StateOpening or StateClosing
	since  time.Time // when moving was set
	byRate bool      // moving was confirmed by device.rate
	// dirty is set when the bridge has published a state that did not come
	// from the FSM (opening/closing/stopped), so the real end state must be
	// re-announced even if the FSM sees no transition.
	dirty bool
	// interrupted is when a safety alert or a part-way stop was last seen.
	interrupted time.Time
	// fsmBusy/fsmSince track how long the FSM has been in opening/closing
	// after a command from Home Assistant.
	fsmBusy  string
	fsmSince time.Time
}

// Observation is one status report for a door.
type Observation struct {
	LogID    int64
	Text     string
	Alert    int
	Position int
	Rate     *float64 // device.rate, nil if the cloud did not send it
}

// Activity describes what an Observation means.
type Activity struct {
	IsNew     bool   // the log entry changed since the last observation
	IsAlert   bool   // the new log entry is a safety/fault event
	Direction string // state to publish now: "", "opening", "closing" or "stopped"
}

// NewMotionTracker creates an empty tracker.
func NewMotionTracker() *MotionTracker {
	return &MotionTracker{devices: make(map[string]*motionInfo)}
}

func (t *MotionTracker) info(deviceID string) (*motionInfo, bool) {
	info, ok := t.devices[deviceID]
	if !ok {
		info = &motionInfo{}
		t.devices[deviceID] = info
	}
	return info, ok
}

// start records that the door is moving in dir and returns dir if that is a
// change worth publishing, or "" if it was already known.
func (i *motionInfo) start(dir string, byRate bool, now time.Time) string {
	if i.moving == dir {
		i.byRate = i.byRate || byRate
		return ""
	}
	i.moving = dir
	i.since = now
	i.byRate = byRate
	i.dirty = true
	return dir
}

func (i *motionInfo) stop(now time.Time) {
	i.moving = ""
	i.byRate = false
	i.dirty = true
	i.interrupted = now
}

func isIntermediate(position int) bool {
	return position > PositionClosed && position < PositionOpen
}

// DirectionFromRate maps device.rate to a direction, ignoring a rate that
// points past the end the door is already at.
func DirectionFromRate(rate float64, position int) string {
	switch {
	case rate > 0 && position < PositionOpen:
		return StateOpening
	case rate < 0 && position > PositionClosed:
		return StateClosing
	}
	return ""
}

// Observe records the latest status for a device and works out what it
// means. The first observation of a device only records the log baseline, so
// an old log entry is not replayed when the bridge starts; device.rate is
// trusted straight away, so a door that is moving (or stopped part-way) when
// the bridge starts is still reported.
func (t *MotionTracker) Observe(deviceID string, o Observation, now time.Time) Activity {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, seen := t.info(deviceID)
	var act Activity
	if !seen {
		info.logID = o.LogID
	} else if o.LogID != 0 && o.LogID != info.logID {
		info.logID = o.LogID
		act.IsNew = true
		act.IsAlert = IsAlertActivity(o.Text, o.Alert)
		if act.IsAlert {
			info.interrupted = now
		}
	}

	// 1. device.rate, when present, is the most direct signal.
	if o.Rate != nil {
		if dir := DirectionFromRate(*o.Rate, o.Position); dir != "" {
			act.Direction = info.start(dir, true, now)
			return act
		}
		// Moves started from the app report rate 0 while travelling, so a new
		// log entry saying the door is opening/closing wins over rate 0.
		startsMove := false
		if act.IsNew && !act.IsAlert {
			d := ClassifyActivity(o.Text, o.Position)
			startsMove = d == StateOpening || d == StateClosing
		}
		if *o.Rate == 0 && isIntermediate(o.Position) && (info.byRate || !seen) && !startsMove {
			// It was moving (or the bridge has just started) and has now
			// halted part-way.
			info.stop(now)
			act.Direction = StateStopped
			return act
		}
	}

	if !act.IsNew {
		return act
	}

	// 2./3. Fall back to the log entry.
	if act.IsAlert {
		// A safety device reversed the door ("Reversing", "Opening by Safety
		// Beam"): it is now travelling back up.
		if IsReversal(o.Text) && o.Position < PositionOpen {
			act.Direction = info.start(StateOpening, false, now)
			return act
		}
		// Otherwise it refused to move or has finished ("Close Prevented as
		// beam is blocked", "Open after Obstruction"). We no longer know
		// which way it is going; dirty stays as it was so a state we
		// published earlier is corrected at the next end position.
		info.moving = ""
		info.byRate = false
		return act
	}

	switch dir := ClassifyActivity(o.Text, o.Position); dir {
	case StateOpening, StateClosing:
		act.Direction = info.start(dir, false, now)
	case StateStopped:
		info.stop(now)
		act.Direction = StateStopped
	}
	return act
}

// Alert wording. Stems match the start of a word ("obstruction",
// "reversing"); exact words must match a whole word, so that short words
// don't fire on names or other words ("Jamie", "Pete", "blockout").
var (
	alertStems = []string{
		"obstruct", "revers", "prevent", "overload", "fault", "error",
		"alarm", "alert", "fail",
	}
	alertWords = map[string]bool{
		"beam": true, "safety": true, "jam": true, "jammed": true,
		"pe": true, "p.e": true, "p.e.": true,
	}
)

func words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.')
	})
}

// IsAlertActivity reports whether a log entry is a safety/fault event: the
// cloud's alert flag is set, or the wording mentions a safety device. The
// wording check is needed because some safety events (e.g. "Opening by
// Safety Beam") arrive with alert 0.
func IsAlertActivity(text string, alert int) bool {
	if alert != 0 {
		return true
	}
	for _, w := range words(text) {
		if alertWords[w] {
			return true
		}
		for _, s := range alertStems {
			if strings.HasPrefix(w, s) {
				return true
			}
		}
	}
	return false
}

// IsReversal reports whether an alert entry means the door has reversed and
// is now opening, as opposed to having finished ("Open after Obstruction") or
// having refused to move ("Close Prevented as beam is blocked").
func IsReversal(text string) bool {
	for _, w := range words(text) {
		if strings.HasPrefix(w, "revers") || w == "opening" {
			return true
		}
	}
	return false
}

// ClassifyActivity works out what a (non-alert) log entry means: "opening",
// "closing", "stopped" (door halted part-way) or "". Past-tense entries
// ("Opened", "Closed") describe a finished movement and are ignored, as is a
// direction the door is already at. With no recognisable wording, the
// direction is inferred from the current end position.
func ClassifyActivity(text string, position int) string {
	t := strings.ToLower(text)
	switch {
	case strings.TrimSpace(t) == "":
		return ""
	case strings.Contains(t, "opened") || strings.Contains(t, "closed"):
		return ""
	case strings.Contains(t, "stop"):
		if isIntermediate(position) {
			return StateStopped
		}
		return ""
	case strings.Contains(t, "clos"):
		if position > PositionClosed {
			return StateClosing
		}
		return ""
	case strings.Contains(t, "open"):
		if position < PositionOpen {
			return StateOpening
		}
		return ""
	}
	// Unknown wording (e.g. "Operated by ..."): infer from where the door is.
	switch position {
	case PositionClosed:
		return StateOpening
	case PositionOpen:
		return StateClosing
	}
	return ""
}

// Moving returns the direction a device is believed to be moving in and for
// how long, or "" if it is not moving.
func (t *MotionTracker) Moving(deviceID string, now time.Time) (string, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, ok := t.devices[deviceID]
	if !ok || info.moving == "" {
		return "", 0
	}
	return info.moving, now.Sub(info.since)
}

// MarkStopped records that the bridge published "stopped" for a door that
// halted part-way.
func (t *MotionTracker) MarkStopped(deviceID string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, ok := t.devices[deviceID]; ok {
		info.stop(now)
	}
}

// Dirty reports whether the bridge has published a state the FSM doesn't
// know about.
func (t *MotionTracker) Dirty(deviceID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, ok := t.devices[deviceID]
	return ok && info.dirty
}

// NeedsResync reports whether the door is at an end position that the FSM
// already considers current, but the bridge has since published a different
// state (opening/closing/stopped) that must be corrected. endState is the
// state the reported position corresponds to ("open" or "closed"). A door that
// has reached the end it was heading for is resynced straight away; one still
// at its starting end is only resynced after MotionGrace, because the
// position isn't always updated until it finishes travelling.
func (t *MotionTracker) NeedsResync(deviceID string, endState string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, ok := t.devices[deviceID]
	if !ok || !info.dirty {
		return false
	}
	if (info.moving == StateOpening && endState == "open") || (info.moving == StateClosing && endState == "closed") {
		return true
	}
	if info.moving != "" && now.Sub(info.since) <= MotionGrace {
		return false
	}
	return true
}

// NoteFSM records the FSM's current state, so that FSMStale can tell how long
// it has been opening/closing.
func (t *MotionTracker) NoteFSM(deviceID, state string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, _ := t.info(deviceID)
	if state != StateOpening && state != StateClosing {
		info.fsmBusy = ""
		return
	}
	if info.fsmBusy != state {
		info.fsmBusy = state
		info.fsmSince = now
	}
}

// FSMStale reports whether an FSM opening/closing state (set by a command
// from Home Assistant) should give way to the reported end position, because
// a safety alert or part-way stop has been seen since, or MotionGrace has
// passed without the door getting there. Without this the FSM stays on
// "closing" forever when, for example, a close is prevented by the beam.
func (t *MotionTracker) FSMStale(deviceID string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, ok := t.devices[deviceID]
	if !ok || info.fsmBusy == "" {
		return false
	}
	if !info.interrupted.IsZero() && !info.interrupted.Before(info.fsmSince) {
		return true
	}
	return now.Sub(info.fsmSince) > MotionGrace
}

// Clear marks a device as settled: not moving and in step with the FSM.
func (t *MotionTracker) Clear(deviceID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, ok := t.devices[deviceID]; ok {
		info.moving = ""
		info.byRate = false
		info.dirty = false
	}
}
