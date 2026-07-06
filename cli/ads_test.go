package main

import (
	"encoding/json"
	"io"
	"testing"
)

func TestImpressionBodyJSONShape(t *testing.T) {
	cfg := &Config{DeviceID: "dev_123"}
	ad := Ad{ID: "ad_456"}

	body, err := io.ReadAll(impressionBody(cfg, ad, "go", 7))
	if err != nil {
		t.Fatal(err)
	}

	want := `{"deviceId":"dev_123","adId":"ad_456","cmd":"go","seconds":7,"kind":"impression"}`
	if string(body) != want {
		t.Fatalf("impression body = %s, want %s", body, want)
	}
}

func TestDeviceRegistrationPayloadJSONShape(t *testing.T) {
	body, err := json.Marshal(deviceRegistrationPayload{
		DeviceID:   "dev_123",
		SecretHash: "abc123",
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `{"deviceId":"dev_123","secretHash":"abc123"}`
	if string(body) != want {
		t.Fatalf("registration body = %s, want %s", body, want)
	}
}

func TestStripControlChars(t *testing.T) {
	in := "safe\n\t" + string(rune(0x7f)) + string(rune(0x80)) + "\u2603"
	got := stripControlChars(in)
	want := "safe\u2603"

	if got != want {
		t.Fatalf("stripControlChars(%q) = %q, want %q", in, got, want)
	}
}
