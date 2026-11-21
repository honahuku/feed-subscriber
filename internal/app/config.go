package app

import (
	"fmt"
	"os"
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
	}
	return config, nil
}

func readYAML(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, destination)
}
