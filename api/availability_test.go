package api

import "testing"

func TestAvailabilityConfig(t *testing.T) {
	c := availabilityConfig("dd-door", "GCQgqF79")
	if c["availability_mode"] != "all" {
		t.Fatalf("availability_mode = %v, want all", c["availability_mode"])
	}
	topics, ok := c["availability"].([]map[string]string)
	if !ok || len(topics) != 2 ||
		topics[0]["topic"] != "dd-door/GCQgqF79/availability" ||
		topics[1]["topic"] != "dd-door/bridge/availability" {
		t.Fatalf("availability = %#v", c["availability"])
	}
	if _, old := c["availability_topic"]; old {
		t.Fatal("availability_topic must not be set alongside availability")
	}
}
