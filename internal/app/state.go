package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type State map[string]string

func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	if state == nil {
		state = State{}
	}
	return state, nil
}

func (state State) Save(path string) error {
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

type Item struct {
	ID          string
	Title       string
	Link        string
	Description string
}

func unseenItems(items []Item, lastID string) ([]Item, string, bool) {
	if len(items) == 0 {
		return nil, lastID, false
	}
	latest := items[0].ID
	if lastID == "" {
		return nil, latest, false
	}
	for index, item := range items {
		if item.ID == lastID {
			return items[:index], latest, true
		}
	}
	return nil, latest, false
}
