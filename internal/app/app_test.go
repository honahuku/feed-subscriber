package app

import "testing"

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
