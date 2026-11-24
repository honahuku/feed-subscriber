package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	maximumRetryInterval   = 24 * time.Hour
	firstFailureAlertDelay = 3 * 24 * time.Hour
	failureAlertInterval   = 3 * 24 * time.Hour
	maximumFailureAlerts   = 4
)

type Failure struct {
	FirstFailedAt       time.Time `json:"first_failed_at"`
	LastFailedAt        time.Time `json:"last_failed_at"`
	NextRetryAt         time.Time `json:"next_retry_at"`
	NextAlertAt         time.Time `json:"next_alert_at"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	AlertCount          int       `json:"alert_count"`
	LastError           string    `json:"last_error"`
}

type FailureState map[string]Failure

func LoadFailureState(path string) (FailureState, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return FailureState{}, nil
	}
	if err != nil {
		return nil, err
	}
	var state FailureState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode RSS failure state: %w", err)
	}
	if state == nil {
		state = FailureState{}
	}
	return state, nil
}

func (state FailureState) Save(path string) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func failureStatePath(statePath string) string {
	return statePath + ".failures.json"
}

func retryInterval(base time.Duration, consecutiveFailures int) time.Duration {
	interval := base
	if interval >= maximumRetryInterval {
		return interval
	}
	for attempt := 1; attempt < consecutiveFailures && interval < maximumRetryInterval; attempt++ {
		if interval > maximumRetryInterval/2 {
			return maximumRetryInterval
		}
		interval *= 2
	}
	if interval > maximumRetryInterval {
		return maximumRetryInterval
	}
	return interval
}
