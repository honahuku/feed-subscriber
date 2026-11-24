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
	config       Config
	state        State
	failureState FailureState
	statePath    string
	client       *http.Client
	parser       *gofeed.Parser
	stateMu      sync.Mutex
	failureMu    sync.Mutex
}

func New(config Config, state State, statePath string) *Application {
	return NewWithFailureState(config, state, FailureState{}, statePath)
}

func NewWithFailureState(config Config, state State, failureState FailureState, statePath string) *Application {
	return &Application{
		config: config, state: state, failureState: failureState, statePath: statePath,
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
		stateURL := feed.StateURLs[urlIndex]
		if failure, ok := application.getFailure(stateURL); ok && time.Now().Before(failure.NextRetryAt) {
			continue
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
		if err != nil {
			if saveErr := application.recordFailure(feed, urlIndex, "invalid RSS request"); saveErr != nil {
				return saveErr
			}
			continue
		}
		response, err := application.client.Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if saveErr := application.recordFailure(feed, urlIndex, "RSS request failed"); saveErr != nil {
				return saveErr
			}
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			message := fmt.Sprintf("RSS request returned HTTP %s", response.Status)
			if err := response.Body.Close(); err != nil {
				return fmt.Errorf("close RSS response: %w", err)
			}
			if saveErr := application.recordFailure(feed, urlIndex, message); saveErr != nil {
				return saveErr
			}
			continue
		}
		parsed, err := application.parser.Parse(response.Body)
		if closeErr := response.Body.Close(); closeErr != nil {
			return fmt.Errorf("close RSS response: %w", closeErr)
		}
		if err != nil {
			if saveErr := application.recordFailure(feed, urlIndex, "unable to parse RSS"); saveErr != nil {
				return saveErr
			}
			continue
		}
		if err := application.clearFailure(stateURL); err != nil {
			return err
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

func (application *Application) recordFailure(feed FeedConfig, urlIndex int, message string) error {
	stateURL := feed.StateURLs[urlIndex]
	now := time.Now()
	application.failureMu.Lock()
	failure := application.failureState[stateURL]
	if failure.FirstFailedAt.IsZero() {
		failure.FirstFailedAt = now
	}
	failure.ConsecutiveFailures++
	failure.LastFailedAt = now
	failure.LastError = message
	failure.NextRetryAt = now.Add(retryInterval(feed.Interval, failure.ConsecutiveFailures))
	if failure.NextAlertAt.IsZero() {
		failure.NextAlertAt = failure.FirstFailedAt.Add(firstFailureAlertDelay)
	}
	application.failureState[stateURL] = failure
	if err := application.failureState.Save(failureStatePath(application.statePath)); err != nil {
		application.failureMu.Unlock()
		return fmt.Errorf("save RSS failure state: %w", err)
	}
	application.failureMu.Unlock()

	logRSSRequestFailure(feed, urlIndex, message)
	return application.sendFailureAlert(stateURL, stateURL, feed, now)
}

func (application *Application) sendFailureAlert(stateURL, displayURL string, feed FeedConfig, now time.Time) error {
	application.failureMu.Lock()
	defer application.failureMu.Unlock()
	failure, exists := application.failureState[stateURL]
	if !exists {
		return nil
	}
	if failure.AlertCount >= maximumFailureAlerts || now.Before(failure.NextAlertAt) {
		return nil
	}
	message := fmt.Sprintf(
		"RSS source failure: feed=%q url=%q error=%q failures=%d",
		feed.Name, displayURL, failure.LastError, failure.ConsecutiveFailures,
	)
	if err := application.sendWebhook(context.Background(), application.config.ErrorWebhookURL, message); err != nil {
		log.Printf("RSS failure alert for %q could not be sent: %v", displayURL, err)
		return nil
	}

	failure.AlertCount++
	failure.NextAlertAt = failure.NextAlertAt.Add(failureAlertInterval)
	application.failureState[stateURL] = failure
	err := application.failureState.Save(failureStatePath(application.statePath))
	if err != nil {
		return fmt.Errorf("save RSS failure alert state: %w", err)
	}
	return nil
}

func (application *Application) getFailure(stateURL string) (Failure, bool) {
	application.failureMu.Lock()
	defer application.failureMu.Unlock()
	failure, ok := application.failureState[stateURL]
	return failure, ok
}

func (application *Application) clearFailure(stateURL string) error {
	application.failureMu.Lock()
	defer application.failureMu.Unlock()
	if _, ok := application.failureState[stateURL]; !ok {
		return nil
	}
	delete(application.failureState, stateURL)
	if err := application.failureState.Save(failureStatePath(application.statePath)); err != nil {
		return fmt.Errorf("save RSS recovery state: %w", err)
	}
	return nil
}

func logRSSRequestFailure(feed FeedConfig, urlIndex int, message string) {
	log.Printf(
		"feed %q (webhook %q), RSS source %q: %s",
		feed.Name,
		feed.Webhook,
		feed.StateURLs[urlIndex],
		message,
	)
}

func (application *Application) saveCheckpoint(key, itemID string) error {
	application.stateMu.Lock()
	defer application.stateMu.Unlock()
	application.state[key] = itemID
	return application.state.Save(application.statePath)
}

func (application *Application) send(ctx context.Context, feed FeedConfig, item Item) error {
	content := formatMessage(feed, item)
	return application.sendWebhook(ctx, feed.WebhookURL, content)
}

func (application *Application) sendWebhook(ctx context.Context, webhookURL, content string) error {
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
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
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
