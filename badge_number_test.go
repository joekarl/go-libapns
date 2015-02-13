package apns

import (
	"encoding/json"
	"testing"
)

func TestBadgeNumberDefaults(t *testing.T) {
	b := BadgeNumber{}

	if b.IsSet() {
		t.Error("BadgeNumber should not be set by default")
	}
	if b.Number() != 0 {
		t.Error("Badge number should be 0 by default")
	}
}

func TestBadgeNumberNew(t *testing.T) {
	b := NewBadgeNumber(5)

	if !b.IsSet() {
		t.Error("NewBadgeNumber should return set BadgeNumber")
	}
	if b.Number() != 5 {
		t.Error("Resulting badge number should be 5")
	}
}

func TestBadgeNumberUnset(t *testing.T) {
	b := NewBadgeNumber(5)

	if !b.IsSet() {
		t.Error("NewBadgeNumber should return set BadgeNumber")
	}

	b.UnSet()

	if b.IsSet() {
		t.Error("UnSet should unset BadgeNumber")
	}
	if b.Number() != 0 {
		t.Error("UnSet should set number to 0")
	}
}

func TestBadgeNumberMarshalJSON(t *testing.T) {
	b := NewBadgeNumber(11)
	m := map[string]BadgeNumber{
		"number": b,
	}

	jsonData, err := json.Marshal(m)
	if err != nil {
		t.Errorf("Error marshalling BadgeNumber: %s", err.Error())
	}

	expected := "{\"number\":11}"
	if string(jsonData) != expected {
		t.Errorf(
			"JSON output\n%s\ndoes not match\n%s",
			string(jsonData),
			expected,
		)
	}
}

func TestBadgeNumberUnmarshalJSON(t *testing.T) {
	type TestStruct struct {
		Number BadgeNumber
	}

	var ts TestStruct
	jsonStr := "{\"number\":11}"
	err := json.Unmarshal([]byte(jsonStr), &ts)
	if err != nil {
		t.Errorf("Error unmarshalling to BadgeNumber: %s", err.Error())
	}

	if !ts.Number.IsSet() {
		t.Error("Resulting BadgeNumber should be set")
	}
	if ts.Number.Number() != 11 {
		t.Error("Expected number to be 11, got %d", ts.Number.Number())
	}
}
