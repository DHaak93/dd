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
// cloud activity log. The SmartDoor cloud adds a log entry (e.g. "Opening by
// Daniel") the moment a door is operated, including from a remote or wall
// button, but the reported position only changes once the door has finished
// travelling. Watching the log lets the bridge publish "opening"/"closing"
// straight away.
type MotionTracker struct {
	mu      sync.Mutex
	devices map[string]*motionInfo
}

type motionInfo struct {
	logID  int64
	moving string // "", "opening" or "closing"
	since  time.Time
}

// NewMotionTracker creates an empty tracker.
func NewMotionTracker() *MotionTracker {
	return &MotionTracker{devices: make(map[string]*motionInfo)}
}

// Observe records the latest log entry for a device. It returns isNew=true
// when the log entry has changed since the last observation, and direction
// ("opening"/"closing") when that new entry indicates the door started moving.
// The first observation of a device only records a baseline.
func (t *MotionTracker) Observe(deviceID string, logID int64, text string, position int, now time.Time) (direction string, isNew bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, seen := t.devices[deviceID]
	if !seen {
		t.devices[deviceID] = &motionInfo{logID: logID}
		return "", false
	}
	if logID == 0 || logID == info.logID {
		return "", false
	}
	info.logID = logID

	direction = ClassifyActivity(text, position)
	if direction != "" {
		info.moving = direction
		info.since = now
	}
	return direction, true
}

// ClassifyActivity works out whether a log entry means the door started
// opening or closing. Past-tense entries ("Opened", "Closed") describe a
// finished movement and are ignored. With no recognisable text, the
// direction is inferred from the current end position.
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

// Clear marks a device as no longer moving.
func (t *MotionTracker) Clear(deviceID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, ok := t.devices[deviceID]; ok {
		info.moving = ""
	}
}
