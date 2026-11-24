package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigResolvesWebhookKeyFromSeparateSecrets(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	secretsPath := filepath.Join(directory, "secrets.yaml")

	config := `error_webhook: updates
feeds:
  - name: engineering
    webhook: updates
    interval: 30m
    description_limit: 10
    urls:
      - https://example.com/feed.xml?token=${secret:feed_token}
`
	secrets := "updates: https://discord.com/api/webhooks/example/token\nfeed_token: private-token\n"
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretsPath, []byte(secrets), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadConfig(configPath, secretsPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(got.Feeds) != 1 {
		t.Fatalf("len(Feeds) = %d, want 1", len(got.Feeds))
	}
	if got.ErrorWebhookURL != "https://discord.com/api/webhooks/example/token" {
		t.Errorf("ErrorWebhookURL = %q, want resolved URL", got.ErrorWebhookURL)
	}
	feed := got.Feeds[0]
	if feed.WebhookURL != "https://discord.com/api/webhooks/example/token" {
		t.Errorf("WebhookURL = %q, want resolved URL", feed.WebhookURL)
	}
	if feed.Interval != 30*time.Minute {
		t.Errorf("Interval = %s, want 30m", feed.Interval)
	}
	if got, want := feed.URLs[0], "https://example.com/feed.xml?token=private-token"; got != want {
		t.Errorf("URLs[0] = %q, want resolved URL", got)
	}
	if got, want := feed.StateURLs[0], "https://example.com/feed.xml?token=${secret:feed_token}"; got != want {
		t.Errorf("StateURLs[0] = %q, want URL template without secret value", got)
	}
}

func TestLoadConfigRejectsMissingURLSecret(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	secretsPath := filepath.Join(directory, "secrets.yaml")

	if err := os.WriteFile(configPath, []byte(`error_webhook: updates
feeds:
  - name: engineering
    webhook: updates
    interval: 30m
    urls: [https://example.com/feed.xml?token=${secret:feed_token}]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretsPath, []byte("updates: https://example.com/webhook\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfig(configPath, secretsPath); err == nil {
		t.Fatal("LoadConfig() error = nil, want missing URL secret error")
	}
}

func TestMigrateStateReplacesURLSecretWithReference(t *testing.T) {
	config := Config{Feeds: []FeedConfig{{
		Name:      "engineering",
		URLs:      []string{"https://example.com/feed.xml?token=private-token"},
		StateURLs: []string{"https://example.com/feed.xml?token=${secret:feed_token}"},
	}}}
	state := State{
		"engineering\nhttps://example.com/feed.xml?token=private-token": "latest-item",
		"unrelated": "unchanged",
	}

	if !config.MigrateState(state) {
		t.Fatal("MigrateState() = false, want migration")
	}
	if got, want := state["engineering\nhttps://example.com/feed.xml?token=${secret:feed_token}"], "latest-item"; got != want {
		t.Errorf("migrated checkpoint = %q, want %q", got, want)
	}
	if _, exists := state["engineering\nhttps://example.com/feed.xml?token=private-token"]; exists {
		t.Error("state still contains URL with secret value")
	}
	if got := state["unrelated"]; got != "unchanged" {
		t.Errorf("unrelated state = %q, want unchanged", got)
	}
}

func TestLoadConfigRejectsMissingWebhookKey(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	secretsPath := filepath.Join(directory, "secrets.yaml")

	if err := os.WriteFile(configPath, []byte(`error_webhook: updates
feeds:
  - name: engineering
    webhook: updates
    interval: 30m
    urls: [https://example.com/feed.xml]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretsPath, []byte("other: https://example.com/webhook\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfig(configPath, secretsPath); err == nil {
		t.Fatal("LoadConfig() error = nil, want missing webhook error")
	}
}

func TestLoadConfigRejectsMissingErrorWebhookKey(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	secretsPath := filepath.Join(directory, "secrets.yaml")

	if err := os.WriteFile(configPath, []byte(`error_webhook: errors
feeds:
  - name: engineering
    webhook: updates
    interval: 30m
    urls: [https://example.com/feed.xml]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretsPath, []byte("updates: https://example.com/webhook\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfig(configPath, secretsPath); err == nil {
		t.Fatal("LoadConfig() error = nil, want missing error webhook error")
	}
}
