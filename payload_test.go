package apns

import (
	"fmt"
	"testing"
)

func TestSimpleMarshal(t *testing.T) {
	p := Payload{
		AlertText:        "Testing this payload",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
	}

	payloadSize := 256

	json, err := p.Marshal(payloadSize)
	if err != nil {
		t.Error(err)
	}

	if len(json) > payloadSize {
		t.Error(fmt.Sprintf("Expected payload to be less than %v but was %v", payloadSize, len(json)))
	}

	expectedJson := "{\"aps\":{\"alert\":\"Testing this payload\",\"badge\":2,\"sound\":\"test.aiff\",\"content-available\":1}}"
	if string(json) != expectedJson {
		t.Error(fmt.Sprintf("Expected %v but got %v", expectedJson, json))
	}
}

func TestSimpleMarshalWithCustomFields(t *testing.T) {
	customFields := map[string]interface{}{
		"num": 55,
		"str": "string",
		"arr": []interface{}{"a", 2},
		"obj": map[string]string{
			"obja": "a",
			"objb": "b",
		},
	}

	p := Payload{
		AlertText:        "Testing this payload",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
	}

	payloadSize := 256

	json, err := p.Marshal(payloadSize)
	if err != nil {
		t.Error(err)
	}

	if len(json) > payloadSize {
		t.Error(fmt.Sprintf("Expected payload to be less than %v but was %v", payloadSize, len(json)))
	}

	expectedJson := "{\"aps\":{\"alert\":\"Testing this payload\",\"badge\":2,\"sound\":\"test.aiff\",\"content-available\":1},\"arr\":[\"a\",2],\"num\":55,\"obj\":{\"obja\":\"a\",\"objb\":\"b\"},\"str\":\"string\"}"
	if string(json) != expectedJson {
		t.Error(fmt.Sprintf("Expected %v but got %v", expectedJson, json))
	}
}

func TestSimpleMarshalTruncate(t *testing.T) {
	p := Payload{
		AlertText: "Testing this payload with a really long message that should " +
			"cause the payload to be truncated yay and stuff blah blah blah blah blah blah " +
			"and some more text to really make this much bigger and stuff",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
	}

	payloadSize := 256

	json, err := p.Marshal(payloadSize)
	if err != nil {
		t.Error(err)
	}

	if len(json) > payloadSize {
		t.Error(fmt.Sprintf("Expected payload to be less than %v but was %v", payloadSize, len(json)))
	}

	expectedJson := "{\"aps\":{\"alert\":\"Testing this payload with a really long message that should cause the payload to be truncated yay and stuff blah blah blah blah blah blah and some more text to really make this much...\",\"badge\":2,\"sound\":\"test.aiff\",\"content-available\":1}}"
	if string(json) != expectedJson {
		t.Error(fmt.Sprintf("Expected %v but got %v", expectedJson, json))
	}
}

func TestSimpleMarshalTruncateWithCustomFields(t *testing.T) {
	customFields := map[string]interface{}{
		"num": 55,
		"str": "string",
		"arr": []interface{}{"a", 2},
		"obj": map[string]string{
			"obja": "a",
			"objb": "b",
		},
	}

	p := Payload{
		AlertText: "Testing this payload with a bunch of text that should get truncated " +
			"so truncate this already please yes thank you blah blah blah blah blah blah " +
			"plus some more text",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
	}

	payloadSize := 256

	json, err := p.Marshal(payloadSize)
	if err != nil {
		t.Error(err)
	}

	if len(json) > payloadSize {
		t.Error(fmt.Sprintf("Expected payload to be less than %v but was %v", payloadSize, len(json)))
	}

	expectedJson := "{\"aps\":{\"alert\":\"Testing this payload with a bunch of text that should get truncated " +
		"so truncate this already please yes thank you...\",\"badge\":2,\"sound\":\"test.aiff\",\"content-available\":1}," +
		"\"arr\":[\"a\",2],\"num\":55,\"obj\":{\"obja\":\"a\",\"objb\":\"b\"},\"str\":\"string\"}"
	if string(json) != expectedJson {
		t.Error(fmt.Sprintf("Expected %v but got %v", expectedJson, json))
	}
}

func TestSimpleMarshalThrowErrorIfPayloadTooBigWithCustomFields(t *testing.T) {
	//lots of custom fields to force failure
	customFields := map[string]interface{}{
		"num": 55,
		"str": "string",
		"arr": []interface{}{"a", 2},
		"obj": map[string]string{
			"obja": "a",
			"objb": "b",
		},
		"obj2": map[string]string{
			"obja": "a",
			"objb": "b",
		},
		"obj3": map[string]string{
			"obja": "a",
			"objb": "b",
		},
		"obj4": map[string]string{
			"obja": "a",
			"objb": "b",
		},
		"obj5": map[string]string{
			"obja": "a",
			"objb": "b",
		},
	}

	p := Payload{
		AlertText:        "Testing this payload",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
	}

	payloadSize := 256

	_, err := p.Marshal(payloadSize)
	if err == nil {
		t.Error("Should have thrown marshaling error")
	}
}

func TestAlertBodyMarshal(t *testing.T) {
	p := Payload{
		AlertText:        "Testing this payload",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		ActionLocKey:     "act-loc-key",
		LocKey:           "loc-key",
		LocArgs:          []string{"arg1", "arg2"},
		LaunchImage:      "launch.png",
	}

	payloadSize := 256

	json, err := p.Marshal(payloadSize)
	if err != nil {
		t.Error(err)
	}

	if len(json) > payloadSize {
		t.Error(fmt.Sprintf("Expected payload to be less than %v but was %v", payloadSize, len(json)))
	}

	expectedJson := "{\"aps\":{\"alert\":{\"body\":\"Testing this payload\",\"action-loc-key\":\"act-loc-key\",\"loc-key\":\"loc-key\",\"loc-args\":[\"arg1\",\"arg2\"],\"launch-image\":\"launch.png\"},\"badge\":2,\"sound\":\"test.aiff\",\"content-available\":1}}"
	if string(json) != expectedJson {
		t.Error(fmt.Sprintf("Expected %v but got %v", expectedJson, json))
	}
}

func TestAlertBodyMarshalWithCustomFields(t *testing.T) {
	customFields := map[string]interface{}{
		"num": 55,
		"str": "string",
		"arr": []interface{}{"a", 2},
		"obj": map[string]string{
			"obja": "a",
			"objb": "b",
		},
	}

	p := Payload{
		AlertText:        "Testing this payload",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
		ActionLocKey:     "act-loc-key",
		LocKey:           "loc-key",
		LaunchImage:      "launch.png",
	}

	payloadSize := 256

	json, err := p.Marshal(payloadSize)
	if err != nil {
		t.Error(err)
	}

	if len(json) > payloadSize {
		t.Error(fmt.Sprintf("Expected payload to be less than %v but was %v", payloadSize, len(json)))
	}

	expectedJson := "{\"aps\":{\"alert\":{\"body\":\"Testing this payload\",\"action-loc-key\":\"act-loc-key\",\"loc-key\":\"loc-key\"," +
		"\"launch-image\":\"launch.png\"}," +
		"\"badge\":2,\"sound\":\"test.aiff\",\"content-available\":1},\"arr\":[\"a\",2]," +
		"\"num\":55,\"obj\":{\"obja\":\"a\",\"objb\":\"b\"},\"str\":\"string\"}"

	if string(json) != expectedJson {
		t.Error(fmt.Sprintf("Expected %v but got %v", expectedJson, json))
	}
}

func TestAlertBodyMarshalTruncate(t *testing.T) {
	p := Payload{
		AlertText: "Testing this payload with a really long message that should " +
			"cause the payload to be truncated yay and stuff blah blah blah blah blah blah " +
			"and some more text to really make this much bigger and stuff",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		LaunchImage:      "launch.png",
	}

	payloadSize := 256

	json, err := p.Marshal(payloadSize)
	if err != nil {
		t.Error(err)
	}

	if len(json) > payloadSize {
		t.Error(fmt.Sprintf("Expected payload to be less than %v but was %v", payloadSize, len(json)))
	}

	expectedJson := "{\"aps\":{\"alert\":{\"body\":\"Testing this payload with a really long message that should cause the payload to be truncated yay and stuff blah blah blah blah blah blah and so...\",\"launch-image\":\"launch.png\"},\"badge\":2,\"sound\":\"test.aiff\",\"content-available\":1}}"
	if string(json) != expectedJson {
		t.Error(fmt.Sprintf("Expected %v but got %v", expectedJson, json))
	}
}

func TestAlertBodyMarshalTruncateWithCustomFields(t *testing.T) {
	customFields := map[string]interface{}{
		"num":  55,
		"str":  "string",
		"arr":  []interface{}{"a", 2},
		"arr2": []interface{}{"a", 2},
	}

	p := Payload{
		AlertText: "Testing this payload with a bunch of text that should get truncated " +
			"so truncate this already please yes thank you blah blah blah blah blah blah " +
			"plus some more text",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
		ActionLocKey:     "act-loc-key",
		LocKey:           "loc-key",
		LocArgs:          []string{"arg1", "arg2"},
		LaunchImage:      "launch.png",
	}

	payloadSize := 256

	json, err := p.Marshal(payloadSize)
	if err != nil {
		t.Error(err)
	}

	if len(json) > payloadSize {
		t.Error(fmt.Sprintf("Expected payload to be less than %v but was %v", payloadSize, len(json)))
	}

	expectedJson := "{\"aps\":{\"alert\":{\"body\":\"Testing this ...\",\"action-loc-key\":\"act-loc-key\",\"loc-key\":\"loc-key\"," +
		"\"loc-args\":[\"arg1\",\"arg2\"],\"launch-image\":\"launch.png\"},\"badge\":2,\"sound\":\"test.aiff\",\"content-available\":1}," +
		"\"arr\":[\"a\",2],\"arr2\":[\"a\",2],\"num\":55,\"str\":\"string\"}"
	if string(json) != expectedJson {
		t.Error(fmt.Sprintf("Expected %v but got %v", expectedJson, json))
	}
}

func TestAlertBodyMarshalThrowErrorIfPayloadTooBigWithCustomFields(t *testing.T) {
	//lots of custom fields to force failure
	customFields := map[string]interface{}{
		"num": 55,
		"str": "string",
		"arr": []interface{}{"a", 2},
		"obj": map[string]string{
			"obja": "a",
			"objb": "b",
		},
		"obj2": map[string]string{
			"obja": "a",
			"objb": "b",
		},
		"obj3": map[string]string{
			"obja": "a",
			"objb": "b",
		},
		"obj4": map[string]string{
			"obja": "a",
			"objb": "b",
		},
		"obj5": map[string]string{
			"obja": "a",
			"objb": "b",
		},
	}

	p := Payload{
		AlertText:        "Testing this payload",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
		LaunchImage:      "launch.png",
	}

	payloadSize := 256

	_, err := p.Marshal(payloadSize)
	if err == nil {
		t.Error("Should have thrown marshaling error")
	}
}

func BenchmarkSimpleMarshalTruncate256WithCustomFields(b *testing.B) {
	customFields := map[string]interface{}{
		"num": 55,
		"str": "string",
		"arr": []interface{}{"a", 2},
		"obj": map[string]string{
			"obja": "a",
			"objb": "b",
		},
	}

	p := Payload{
		AlertText: "Testing this payload with a bunch of text that should get truncated " +
			"so truncate this already please yes thank you blah blah blah blah blah blah " +
			"plus some more text",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Marshal(256)
	}
}

func BenchmarkSimpleMarshalTruncate1024WithCustomFields(b *testing.B) {
	customFields := map[string]interface{}{
		"num": 55,
		"str": "string",
		"arr": []interface{}{"a", 2},
		"obj": map[string]string{
			"obja": "a",
			"objb": "b",
		},
	}

	p := Payload{
		AlertText: "Testing this payload with a bunch of text that should get truncated " +
			"so truncate this already please yes thank you blah blah blah blah blah blah " +
			"plus some more text",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Marshal(1024)
	}
}

func BenchmarkAlertBodyMarshalTruncate256WithCustomFields(b *testing.B) {
	customFields := map[string]interface{}{
		"num": 55,
		"str": "string",
		"arr": []interface{}{"a", 2},
		"obj": map[string]string{
			"obja": "a",
			"objb": "b",
		},
	}

	p := Payload{
		AlertText: "Testing this payload with a bunch of text that should get truncated " +
			"so truncate this already please yes thank you blah blah blah blah blah blah " +
			"plus some more text",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
		LaunchImage:      "launch.png",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Marshal(256)
	}
}

func BenchmarkAlertBodyMarshalTruncate1024WithCustomFields(b *testing.B) {
	customFields := map[string]interface{}{
		"num": 55,
		"str": "string",
		"arr": []interface{}{"a", 2},
		"obj": map[string]string{
			"obja": "a",
			"objb": "b",
		},
	}

	p := Payload{
		AlertText: "Testing this payload with a bunch of text that should get truncated " +
			"so truncate this already please yes thank you blah blah blah blah blah blah " +
			"plus some more text",
		Badge:            2,
		ContentAvailable: 1,
		Sound:            "test.aiff",
		CustomFields:     customFields,
		LaunchImage:      "launch.png",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Marshal(1024)
	}
}
