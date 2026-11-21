package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Feeds []FeedConfig `yaml:"feeds"`
}

type FeedConfig struct {
	Name             string        `yaml:"name"`
	Webhook          string        `yaml:"webhook"`
	Interval         time.Duration `yaml:"-"`
	IntervalText     string        `yaml:"interval"`
	DescriptionLimit int           `yaml:"description_limit"`
	URLs             []string      `yaml:"urls"`
	StateURLs        []string      `yaml:"-"`
	WebhookURL       string        `yaml:"-"`
}

func LoadConfig(configPath, secretsPath string) (Config, error) {
	var config Config
	if err := readYAML(configPath, &config); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var secrets map[string]string
	if err := readYAML(secretsPath, &secrets); err != nil {
		return Config{}, fmt.Errorf("read secrets: %w", err)
	}
	if len(config.Feeds) == 0 {
		return Config{}, fmt.Errorf("config must contain at least one feed")
	}

	names := make(map[string]bool)
	for i := range config.Feeds {
		feed := &config.Feeds[i]
		if feed.Name == "" || names[feed.Name] {
			return Config{}, fmt.Errorf("feed names must be non-empty and unique")
		}
		names[feed.Name] = true

		interval, err := time.ParseDuration(feed.IntervalText)
		if err != nil || interval <= 0 {
			return Config{}, fmt.Errorf("feed %q has invalid interval %q", feed.Name, feed.IntervalText)
		}
		feed.Interval = interval
		if feed.Webhook == "" || secrets[feed.Webhook] == "" {
			return Config{}, fmt.Errorf("feed %q references missing webhook key %q", feed.Name, feed.Webhook)
		}
		feed.WebhookURL = secrets[feed.Webhook]
		if len(feed.URLs) == 0 {
			return Config{}, fmt.Errorf("feed %q must have at least one URL", feed.Name)
		}
		feed.StateURLs = append([]string(nil), feed.URLs...)
		for urlIndex, urlTemplate := range feed.StateURLs {
			resolvedURL, err := resolveSecretReferences(urlTemplate, secrets)
			if err != nil {
				return Config{}, fmt.Errorf("feed %q URL %d: %w", feed.Name, urlIndex+1, err)
			}
			feed.URLs[urlIndex] = resolvedURL
		}
	}
	return config, nil
}

func resolveSecretReferences(value string, secrets map[string]string) (string, error) {
	const referencePrefix = "${secret:"
	for {
		start := strings.Index(value, referencePrefix)
		if start == -1 {
			return value, nil
		}
		endOffset := strings.IndexByte(value[start+len(referencePrefix):], '}')
		if endOffset == -1 {
			return "", fmt.Errorf("unterminated secret reference")
		}
		end := start + len(referencePrefix) + endOffset
		key := value[start+len(referencePrefix) : end]
		secret, ok := secrets[key]
		if key == "" || !ok || secret == "" {
			return "", fmt.Errorf("missing URL secret %q", key)
		}
		value = value[:start] + secret + value[end+1:]
	}
}

func (config Config) MigrateState(state State) bool {
	changed := false
	for _, feed := range config.Feeds {
		for urlIndex, feedURL := range feed.URLs {
			oldKey := feed.Name + "\n" + feedURL
			newKey := feed.Name + "\n" + feed.StateURLs[urlIndex]
			if oldKey == newKey {
				continue
			}
			if lastID, ok := state[oldKey]; ok {
				if _, exists := state[newKey]; !exists {
					state[newKey] = lastID
				}
				delete(state, oldKey)
				changed = true
			}
		}
	}
	return changed
}

func readYAML(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, destination)
}
