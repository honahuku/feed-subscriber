package app

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFormatMessageContainsOnlyArticleURL(t *testing.T) {
	feed := FeedConfig{DescriptionLimit: 5}
	item := Item{
		Title:       "Release",
		Link:        "https://example.com/release",
		Description: "A longer summary",
	}

	got := formatMessage(feed, item)
	want := "https://example.com/release"
	if got != want {
		t.Errorf("formatMessage() = %q, want %q", got, want)
	}
}

func TestPollLogsWebhookAndRSSURLOnHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	var output bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(originalOutput)

	feed := FeedConfig{
		Name:      "test server",
		Webhook:   "updates",
		URLs:      []string{server.URL + "/feed?token=private-token"},
		StateURLs: []string{"https://feeds.example.org/rss?token=${secret:feed_token}"},
	}
	application := New(Config{}, State{}, filepath.Join(t.TempDir(), "state.json"))
	if err := application.poll(context.Background(), feed); err != nil {
		t.Fatalf("poll() error = %v, want nil", err)
	}

	got := output.String()
	for _, want := range []string{
		`feed "test server"`,
		`webhook "updates"`,
		`https://feeds.example.org/rss?token=${secret:feed_token}`,
		"HTTP 503 Service Unavailable",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("poll() log = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "private-token") {
		t.Errorf("poll() log contains the resolved URL secret: %q", got)
	}
}

func TestSendFailureAlertSendsAtDueTimeAndStopsAfterFourAlerts(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	now := time.Now()
	failure := Failure{
		FirstFailedAt:       now.Add(-firstFailureAlertDelay),
		NextAlertAt:         now,
		ConsecutiveFailures: 10,
		LastError:           "HTTP 503",
	}
	application := NewWithFailureState(
		Config{ErrorWebhookURL: server.URL},
		State{},
		FailureState{"https://example.com/feed.xml": failure},
		filepath.Join(t.TempDir(), "state.json"),
	)
	feed := FeedConfig{Name: "test"}

	for attempt := range maximumFailureAlerts + 1 {
		alertAt := now.Add(time.Duration(attempt) * failureAlertInterval)
		if err := application.sendFailureAlert("https://example.com/feed.xml", "https://example.com/feed.xml", feed, alertAt); err != nil {
			t.Fatalf("sendFailureAlert() error = %v", err)
		}
	}

	if requests != maximumFailureAlerts {
		t.Errorf("webhook requests = %d, want %d", requests, maximumFailureAlerts)
	}
	got, _ := application.getFailure("https://example.com/feed.xml")
	if got.AlertCount != maximumFailureAlerts {
		t.Errorf("AlertCount = %d, want %d", got.AlertCount, maximumFailureAlerts)
	}
}

func TestPollClearsFailureAfterSuccessfulParse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/rss+xml")
		_, _ = writer.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>test</title><link>https://example.com</link><description>test</description></channel></rss>`))
	}))
	defer server.Close()

	stateURL := "https://example.com/feed.xml"
	application := NewWithFailureState(
		Config{},
		State{},
		FailureState{stateURL: {
			FirstFailedAt:       time.Now().Add(-time.Hour),
			NextRetryAt:         time.Now().Add(-time.Minute),
			ConsecutiveFailures: 3,
			LastError:           "HTTP 503",
		}},
		filepath.Join(t.TempDir(), "state.json"),
	)
	feed := FeedConfig{
		Name: "test", Interval: time.Minute,
		URLs:      []string{server.URL},
		StateURLs: []string{stateURL},
	}
	if err := application.poll(context.Background(), feed); err != nil {
		t.Fatalf("poll() error = %v", err)
	}
	if _, exists := application.getFailure(stateURL); exists {
		t.Error("failure state remains after successful RSS parse")
	}
}
