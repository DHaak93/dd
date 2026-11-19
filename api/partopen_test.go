package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPartOpenCommand(t *testing.T) {
	for payload, want := range map[string]int{"part_open_1": 5, "PART_OPEN_2": 6, " part_open_3 ": 7} {
		if got, ok := PartOpenCommand(payload); !ok || got != want {
			t.Errorf("PartOpenCommand(%q) = %d, %v; want %d", payload, got, ok, want)
		}
	}
	for _, payload := range []string{"", "GO_OPEN", "part_open_4", "open"} {
		if _, ok := PartOpenCommand(payload); ok {
			t.Errorf("PartOpenCommand(%q) should not match", payload)
		}
	}
}

func TestPartOpenButtonsUsesDoorTitles(t *testing.T) {
	raw := `[{"action":{"base":0,"cmd":2},"title":"Open"},
		{"action":{"base":0,"cmd":5},"title":"Pet"},
		{"action":{"base":0,"cmd":6},"title":"Parcel"},
		{"action":{"base":0,"cmd":7},"title":"Ventilation","hide":1}]`
	var buttons []DoorStatusButton
	if err := json.Unmarshal([]byte(raw), &buttons); err != nil {
		t.Fatal(err)
	}
	got := PartOpenButtons(buttons)
	if len(got) != 2 || got[0].Title != "Pet" || got[0].Key != "part_open_1" || got[1].Title != "Parcel" || got[1].Command != 6 {
		t.Fatalf("got %+v", got)
	}
}

func TestPartOpenButtonsFallback(t *testing.T) {
	for _, buttons := range [][]DoorStatusButton{nil, {{Title: "Open"}}} {
		got := PartOpenButtons(buttons)
		if len(got) != 3 || got[0].Title != "Partial open 1" || got[2].Command != 7 {
			t.Fatalf("fallback: got %+v", got)
		}
	}
}

// Pet from the app, recorded on the front door: shown as opening, then open
// where it stopped (78%).
func TestPetPreset(t *testing.T) {
	tr := NewMotionTracker()
	now := time.Now()
	tr.Observe("front", Observation{LogID: 1, Text: "Closed", Position: 0, Rate: rate(0)}, now)
	if a := tr.Observe("front", Observation{LogID: 2, Text: "Daniel sent Pet", Position: 0, Rate: rate(0)}, now); a.Direction != StateOpening {
		t.Fatalf("pet: got %+v", a)
	}
}
