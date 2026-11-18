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

// MotionTracker detects when a door starts moving by watching each device's
// cloud activity log. The SmartDoor cloud adds a log entry (e.g. "Daniel sent
// Open") the moment a door is operated, but the reported position only
// changes once the door has finished travelling. Watching the log lets the
// bridge publish "opening"/"closing" straight away.
type MotionTracker struct {
	mu      sync.Mutex
	devices map[string]*motionInfo
}

type motionInfo struct {
	logID  int64
	moving string // "", "opening" or "closing"
	since  time.Time
	// dirty is set when the bridge has published a state that did not come
	// from the FSM (opening/closing/stopped), so the real end state must be
	// re-announced even if the FSM sees no transition.
	dirty bool
}

// Activity describes a new cloud log entry for a device.
type Activity struct {
	IsNew     bool   // the log entry changed since the last observation
	Direction string // "opening"/"closing" if the entry means the door started moving
	IsAlert   bool   // safety/fault entry (PE beam, obstruction, ...)
}

// NewMotionTracker creates an empty tracker.
func NewMotionTracker() *MotionTracker {
	return &MotionTracker{devices: make(map[string]*motionInfo)}
}

// Observe records the latest log entry for a device and works out what it
// means. The first observation of a device only records a baseline.
func (t *MotionTracker) Observe(deviceID string, logID int64, text string, alert int, position int, now time.Time) Activity {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, seen := t.devices[deviceID]
	if !seen {
		t.devices[deviceID] = &motionInfo{logID: logID}
		return Activity{}
	}
	if logID == 0 || logID == info.logID {
		return Activity{}
	}
	info.logID = logID

	act := Activity{IsNew: true, IsAlert: IsAlertActivity(text, alert)}
	if act.IsAlert {
		// The door may have stopped or reversed; we no longer know which way
		// it is going. Leave dirty set so the real state is re-announced at
		// the next end position.
		info.moving = ""
		return act
	}

	act.Direction = ClassifyActivity(text, position)
	if act.Direction != "" {
		info.moving = act.Direction
		info.since = now
		info.dirty = true
	}
	return act
}

// alertWords mark log entries about safety devices or faults rather than a
// door being operated.
var alertWords = []string{
	"beam", "p.e", "pe ", "obstruct", "block", "safety", "revers",
	"fault", "error", "jam", "overload", "fail", "alarm", "alert",
}

// IsAlertActivity reports whether a log entry is a safety/fault event. It
// goes by the wording only: the meaning of the cloud's numeric "alert" field
// is not known yet, so it is logged but not relied on.
func IsAlertActivity(text string, alert int) bool {
	_ = alert
	t := strings.ToLower(text) + " "
	for _, w := range alertWords {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
}

// ClassifyActivity works out whether a log entry means the door started
// opening or closing. Past-tense entries ("Opened", "Closed") describe a
// finished movement and are ignored. With no recognisable wording, the
// direction is inferred from the current end position. Alert entries must be
// filtered out with IsAlertActivity before calling this.
func ClassifyActivity(text string, position int) string {
	t := strings.ToLower(text)
	if strings.Contains(t, "opened") || strings.Contains(t, "closed") {
		return ""
	}
	switch {
	case strings.Contains(t, "clos"):
		if position > PositionClosed {
			return "closing"
		}
		return ""
	case strings.Contains(t, "open"):
		if position < PositionOpen {
			return "opening"
		}
		return ""
	case strings.TrimSpace(t) == "":
		return ""
	}
	// Unknown wording (e.g. "Operated by ..."): infer from where the door is.
	switch position {
	case PositionClosed:
		return "opening"
	case PositionOpen:
		return "closing"
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
func (t *MotionTracker) MarkStopped(deviceID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, ok := t.devices[deviceID]; ok {
		info.moving = ""
		info.dirty = true
	}
}

// NeedsResync reports whether the door is at an end position that the FSM
// already considers current, but the bridge has since published a different
// state (opening/closing/stopped) that must be corrected. A door still within
// MotionGrace of starting to move is not resynced, because the position
// isn't updated until it finishes travelling.
func (t *MotionTracker) NeedsResync(deviceID string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, ok := t.devices[deviceID]
	if !ok || !info.dirty {
		return false
	}
	if info.moving != "" && now.Sub(info.since) <= MotionGrace {
		return false
	}
	return true
}

// Clear marks a device as settled: not moving and in step with the FSM.
func (t *MotionTracker) Clear(deviceID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, ok := t.devices[deviceID]; ok {
		info.moving = ""
		info.dirty = false
	}
}
