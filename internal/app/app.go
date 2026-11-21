package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

type Application struct {
	config    Config
	state     State
	statePath string
	client    *http.Client
	parser    *gofeed.Parser
	stateMu   sync.Mutex
}

func New(config Config, state State, statePath string) *Application {
	return &Application{
		config: config, state: state, statePath: statePath,
		client: &http.Client{Timeout: 20 * time.Second},
		parser: gofeed.NewParser(),
	}
}

func (application *Application) Run(ctx context.Context) error {
	errors := make(chan error, len(application.config.Feeds))
	for _, feed := range application.config.Feeds {
		go application.runFeed(ctx, feed, errors)
	}
	for range application.config.Feeds {
		if err := <-errors; err != nil {
			return err
		}
	}
	return nil
}

func (application *Application) runFeed(ctx context.Context, feed FeedConfig, errors chan<- error) {
	if err := application.poll(ctx, feed); err != nil {
		errors <- fmt.Errorf("feed %q: %w", feed.Name, err)
		return
	}
	ticker := time.NewTicker(feed.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			errors <- nil
			return
		case <-ticker.C:
			if err := application.poll(ctx, feed); err != nil {
				errors <- fmt.Errorf("feed %q: %w", feed.Name, err)
				return
			}
		}
	}
}

func (application *Application) poll(ctx context.Context, feed FeedConfig) error {
	for urlIndex, feedURL := range feed.URLs {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
		if err != nil {
			log.Printf("feed %q: invalid RSS request", feed.Name)
			continue
		}
		response, err := application.client.Do(request)
		if err != nil {
			log.Printf("feed %q: RSS request failed", feed.Name)
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			log.Printf("feed %q: RSS request returned HTTP %s", feed.Name, response.Status)
			if err := response.Body.Close(); err != nil {
				return fmt.Errorf("close RSS response: %w", err)
			}
			continue
		}
		parsed, err := application.parser.Parse(response.Body)
		if closeErr := response.Body.Close(); closeErr != nil {
			return fmt.Errorf("close RSS response: %w", closeErr)
		}
		if err != nil {
			log.Printf("feed %q: unable to parse RSS", feed.Name)
			continue
		}
		items := make([]Item, 0, len(parsed.Items))
		for _, parsedItem := range parsed.Items {
			id := parsedItem.GUID
			if id == "" {
				id = parsedItem.Link
			}
			if id == "" {
				continue
			}
			items = append(items, Item{
				ID: id, Title: parsedItem.Title, Link: parsedItem.Link,
				Description: parsedItem.Description,
			})
		}
		key := feed.Name + "\n" + feed.StateURLs[urlIndex]
		application.stateMu.Lock()
		lastID := application.state[key]
		application.stateMu.Unlock()
		newItems, latest, found := unseenItems(items, lastID)
		if latest != "" && (lastID == "" || !found) {
			if err := application.saveCheckpoint(key, latest); err != nil {
				return err
			}
			continue
		}
		for index := len(newItems) - 1; index >= 0; index-- {
			if err := application.send(ctx, feed, newItems[index]); err != nil {
				return err
			}
			if err := application.saveCheckpoint(key, newItems[index].ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (application *Application) saveCheckpoint(key, itemID string) error {
	application.stateMu.Lock()
	defer application.stateMu.Unlock()
	application.state[key] = itemID
	return application.state.Save(application.statePath)
}

func (application *Application) send(ctx context.Context, feed FeedConfig, item Item) error {
	content := formatMessage(feed, item)
	payload := struct {
		Content         string `json:"content"`
		AllowedMentions struct {
			Parse []string `json:"parse"`
		} `json:"allowed_mentions"`
	}{Content: content}
	payload.AllowedMentions.Parse = []string{}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, feed.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := application.client.Do(request)
	if err != nil {
		return err
	}
	if err := response.Body.Close(); err != nil {
		return fmt.Errorf("close webhook response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %s", response.Status)
	}
	return nil
}

func formatMessage(_ FeedConfig, item Item) string {
	return item.Link
}
