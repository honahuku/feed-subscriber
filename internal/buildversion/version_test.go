package buildversion

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStampUsesCurrentTimeWhenNewer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VERSION")
	now := time.Date(2026, time.November, 22, 1, 2, 3, 456789000, time.FixedZone("JST", 9*60*60))

	if err := Stamp(path, now); err != nil {
		t.Fatalf("Stamp() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := "20261121160203.456\n"
	if string(got) != want {
		t.Errorf("version = %q, want %q", got, want)
	}
}

func TestStampAdvancesWhenTimeWouldRepeatOrGoBackwards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(path, []byte("20261122160203.456\n"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	now := time.Date(2026, time.November, 22, 16, 2, 3, 456000000, time.UTC)

	if err := Stamp(path, now); err != nil {
		t.Fatalf("Stamp() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := "20261122160203.457\n"
	if string(got) != want {
		t.Errorf("version = %q, want %q", got, want)
	}
}
