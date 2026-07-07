package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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

func TestFetchAdDebugLogsServeHTTPFailure(t *testing.T) {
	t.Setenv("BACKFILL_DEBUG", "1")
	withHTTPClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("unavailable")),
			Header:     make(http.Header),
		}, nil
	}))

	cfg := &Config{APIBase: "https://api.example.test", DeviceID: "dev_test"}
	out := captureStderr(t, func() {
		ad := fetchAd(cfg, "go")
		if ad.ID == "" || ad.Text == "" {
			t.Fatalf("fetchAd did not return fallback ad: %+v", ad)
		}
	})

	want := `bf debug: ad serve returned HTTP 503 for cmd "go"`
	if !strings.Contains(out, want) {
		t.Fatalf("debug log = %q, want substring %q", out, want)
	}
	if strings.Contains(out, "dev_test") || strings.Contains(out, "api.example.test") {
		t.Fatalf("debug log exposed device id or URL: %q", out)
	}
}

func TestPostImpressionDebugLogsHTTPFailure(t *testing.T) {
	t.Setenv("BACKFILL_DEBUG", "1")
	withHTTPClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("try later")),
			Header:     make(http.Header),
		}, nil
	}))

	cfg := &Config{APIBase: "https://api.example.test", DeviceID: "dev_test"}
	out := captureStderr(t, func() {
		postImpression(cfg, Ad{ID: "ad_test"}, "go", 7)
	})

	want := `bf debug: impression report returned HTTP 500 for cmd "go"`
	if !strings.Contains(out, want) {
		t.Fatalf("debug log = %q, want substring %q", out, want)
	}
	if strings.Contains(out, "dev_test") || strings.Contains(out, "ad_test") || strings.Contains(out, "api.example.test") {
		t.Fatalf("debug log exposed device id, ad id, or URL: %q", out)
	}
}

func TestRegisterDeviceDebugLogsHTTPFailure(t *testing.T) {
	t.Setenv("BACKFILL_DEBUG", "1")
	withHTTPClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("bad gateway")),
			Header:     make(http.Header),
		}, nil
	}))

	cfg := &Config{APIBase: "https://api.example.test", DeviceID: "dev_test", DeviceSecret: "secret_test"}
	out := captureStderr(t, func() {
		registerDevice(cfg)
	})

	want := `bf debug: device registration returned HTTP 502`
	if !strings.Contains(out, want) {
		t.Fatalf("debug log = %q, want substring %q", out, want)
	}
	if strings.Contains(out, "dev_test") || strings.Contains(out, "secret_test") || strings.Contains(out, "api.example.test") {
		t.Fatalf("debug log exposed device id, secret, or URL: %q", out)
	}
}

func TestDebugLogQuietByDefault(t *testing.T) {
	t.Setenv("BACKFILL_DEBUG", "")
	out := captureStderr(t, func() {
		debugLogf("hidden message")
	})
	if out != "" {
		t.Fatalf("debug log should be quiet by default, got %q", out)
	}
}

func TestSafeHTTPErrorOmitsRawURL(t *testing.T) {
	err := &url.Error{
		Op:  "Get",
		URL: "https://api.example.test/api/serve?d=dev_test",
		Err: errors.New("lookup api.example.test"),
	}

	got := safeHTTPError(err)
	if !strings.HasPrefix(got, "Get: ") {
		t.Fatalf("safeHTTPError = %q, want operation prefix", got)
	}
	if strings.Contains(got, "api.example.test") || strings.Contains(got, "dev_test") {
		t.Fatalf("safeHTTPError exposed URL details: %q", got)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = old
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func withHTTPClient(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	old := httpClient
	httpClient = &http.Client{Transport: rt}
	t.Cleanup(func() {
		httpClient = old
	})
}
