package agent

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func TestResultStorePersistsAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	cfg := resultStoreConfig{Path: dir, Retention: 100}

	store, err := newResultStore(cfg)
	if err != nil {
		t.Fatalf("initial store: %v", err)
	}

	expected := []protocol.CommandResult{
		{CommandID: "cmd-1"},
		{CommandID: "cmd-2"},
	}
	if err := store.AppendAll(expected); err != nil {
		t.Fatalf("append results: %v", err)
	}

	reopened, err := newResultStore(cfg)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}

	results, err := reopened.All()
	if err != nil {
		t.Fatalf("read results: %v", err)
	}
	if len(results) != len(expected) {
		t.Fatalf("unexpected result count: got %d want %d", len(results), len(expected))
	}
	for i := range expected {
		if results[i].CommandID != expected[i].CommandID {
			t.Fatalf("result %d mismatch: got %q want %q", i, results[i].CommandID, expected[i].CommandID)
		}
	}
}

func TestDefaultResultStorePathUsesDataDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	pref := BuildPreferences{}
	path := defaultResultStorePath(pref)
	expected := filepath.Join(tmp, ".config", "tenvy", "results")
	if path != expected {
		t.Fatalf("expected result store path %s, got %s", expected, path)
	}
}

func TestDefaultResultStorePathCustomBranding(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	pref := BuildPreferences{
		Persistence: PersistenceBranding{BaseDataDir: filepath.Join(".data", "custom")},
	}

	path := defaultResultStorePath(pref)
	expected := filepath.Join(tmp, ".data", "custom", "results")
	if path != expected {
		t.Fatalf("expected custom result store path %s, got %s", expected, path)
	}
}

func TestResultStoreEvictsBeyondRetention(t *testing.T) {
	dir := t.TempDir()
	cfg := resultStoreConfig{Path: dir, Retention: 3}

	store, err := newResultStore(cfg)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	total := 5
	for i := 0; i < total; i++ {
		result := protocol.CommandResult{CommandID: fmt.Sprintf("cmd-%d", i)}
		if err := store.Append(result); err != nil {
			t.Fatalf("append result %d: %v", i, err)
		}
	}

	results, err := store.All()
	if err != nil {
		t.Fatalf("read results: %v", err)
	}
	if len(results) != cfg.Retention {
		t.Fatalf("unexpected retained results: got %d want %d", len(results), cfg.Retention)
	}
	for idx, result := range results {
		expectedID := fmt.Sprintf("cmd-%d", total-cfg.Retention+idx)
		if result.CommandID != expectedID {
			t.Fatalf("unexpected result at index %d: got %q want %q", idx, result.CommandID, expectedID)
		}
	}
}
