package buildversion

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const layout = "20060102150405.000"

func Stamp(path string, now time.Time) error {
	now = now.UTC().Truncate(time.Millisecond)
	previous, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read version: %w", err)
	}
	if err == nil {
		previousTime, parseErr := time.Parse(layout, strings.TrimSpace(string(previous)))
		if parseErr == nil && !now.After(previousTime) {
			now = previousTime.Add(time.Millisecond)
		}
	}
	if err := os.WriteFile(path, []byte(now.Format(layout)+"\n"), 0644); err != nil {
		return fmt.Errorf("write version: %w", err)
	}
	return nil
}
