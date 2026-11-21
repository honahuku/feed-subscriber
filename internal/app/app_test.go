package app

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	application := New(Config{}, State{}, "")
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
