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

	config := `feeds:
  - name: engineering
    webhook: updates
    interval: 30m
    description_limit: 10
    urls:
      - https://example.com/feed.xml
`
	secrets := "updates: https://discord.com/api/webhooks/example/token\n"
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
	feed := got.Feeds[0]
	if feed.WebhookURL != "https://discord.com/api/webhooks/example/token" {
		t.Errorf("WebhookURL = %q, want resolved URL", feed.WebhookURL)
	}
	if feed.Interval != 30*time.Minute {
		t.Errorf("Interval = %s, want 30m", feed.Interval)
	}
}

func TestLoadConfigRejectsMissingWebhookKey(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	secretsPath := filepath.Join(directory, "secrets.yaml")

	if err := os.WriteFile(configPath, []byte(`feeds:
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
