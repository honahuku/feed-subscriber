package app

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRetryIntervalDoublesAndCapsAtOneDay(t *testing.T) {
	tests := []struct {
		base     time.Duration
		failures int
		want     time.Duration
	}{
		{base: 15 * time.Minute, failures: 1, want: 15 * time.Minute},
		{base: 15 * time.Minute, failures: 2, want: 30 * time.Minute},
		{base: 15 * time.Minute, failures: 3, want: time.Hour},
		{base: 15 * time.Minute, failures: 8, want: maximumRetryInterval},
		{base: 48 * time.Hour, failures: 1, want: 48 * time.Hour},
	}
	for _, test := range tests {
		if got := retryInterval(test.base, test.failures); got != test.want {
			t.Errorf("retryInterval(%s, %d) = %s, want %s", test.base, test.failures, got, test.want)
		}
	}
}

func TestFailureStateRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.failures.json")
	want := FailureState{
		"https://example.com/feed.xml": {
			FirstFailedAt:       time.Date(2026, time.November, 25, 0, 0, 0, 0, time.UTC),
			NextRetryAt:         time.Date(2026, time.November, 25, 0, 30, 0, 0, time.UTC),
			ConsecutiveFailures: 2,
			LastError:           "HTTP 503",
		},
	}
	if err := want.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := LoadFailureState(path)
	if err != nil {
		t.Fatalf("LoadFailureState() error = %v", err)
	}
	if got["https://example.com/feed.xml"] != want["https://example.com/feed.xml"] {
		t.Errorf("loaded failure = %#v, want %#v", got["https://example.com/feed.xml"], want["https://example.com/feed.xml"])
	}
}
