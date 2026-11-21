package app

import (
	"strings"
	"testing"
)

func TestFormatMessageUsesMarkdownAndLimitsDescription(t *testing.T) {
	feed := FeedConfig{DescriptionLimit: 5}
	item := Item{
		Title:       "Release",
		Link:        "https://example.com/release",
		Description: "A longer summary",
	}

	got := formatMessage(feed, item)
	want := "[Release](https://example.com/release)\n\nA lon…"
	if got != want {
		t.Errorf("formatMessage() = %q, want %q", got, want)
	}
}

func TestFormatMessageRespectsDiscordLengthLimit(t *testing.T) {
	got := formatMessage(FeedConfig{}, Item{Title: strings.Repeat("x", 2500)})
	if len([]rune(got)) != 2000 {
		t.Errorf("message length = %d runes, want 2000", len([]rune(got)))
	}
}
