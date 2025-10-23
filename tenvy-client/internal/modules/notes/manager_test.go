package notes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerMigratesLegacyLocalNotes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.json")

	legacyLocal := "legacy-local-key"
	sharedMaterial := "shared-key"

	legacyManager, err := NewManager(path, legacyLocal, sharedMaterial, "")
	if err != nil {
		t.Fatalf("failed to initialize legacy manager: %v", err)
	}

	note, err := legacyManager.SaveNote(Note{Title: "Legacy", Body: "classified", Shared: false}, 0)
	if err != nil {
		t.Fatalf("failed to save legacy note: %v", err)
	}

	rawBefore, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read notes file: %v", err)
	}

	var snapshotBefore noteFile
	if err := json.Unmarshal(rawBefore, &snapshotBefore); err != nil {
		t.Fatalf("failed to decode notes file: %v", err)
	}

	if len(snapshotBefore.Notes) != 1 {
		t.Fatalf("expected 1 note in snapshot, got %d", len(snapshotBefore.Notes))
	}

	originalCiphertext := snapshotBefore.Notes[0].Ciphertext

	stableLocal := "stable-local-key"
	migratedManager, err := NewManager(path, stableLocal, sharedMaterial, legacyLocal)
	if err != nil {
		t.Fatalf("failed to initialize migrated manager: %v", err)
	}

	listed, err := migratedManager.ListNotes()
	if err != nil {
		t.Fatalf("failed to list notes: %v", err)
	}

	if len(listed) != 1 {
		t.Fatalf("expected 1 note after migration, got %d", len(listed))
	}

	got := listed[0]
	if got.ID != note.ID || got.Title != note.Title || got.Body != note.Body {
		t.Fatalf("unexpected note content after migration: %+v", got)
	}

	rawAfter, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read notes file after migration: %v", err)
	}

	var snapshotAfter noteFile
	if err := json.Unmarshal(rawAfter, &snapshotAfter); err != nil {
		t.Fatalf("failed to decode migrated notes file: %v", err)
	}

	if len(snapshotAfter.Notes) != 1 {
		t.Fatalf("expected 1 note in migrated snapshot, got %d", len(snapshotAfter.Notes))
	}

	if snapshotAfter.Notes[0].Ciphertext == originalCiphertext {
		t.Fatalf("expected ciphertext to change after migration")
	}

	stableManager, err := NewManager(path, stableLocal, sharedMaterial, "")
	if err != nil {
		t.Fatalf("failed to initialize stable manager: %v", err)
	}

	stableNotes, err := stableManager.ListNotes()
	if err != nil {
		t.Fatalf("failed to list notes with stable key: %v", err)
	}

	if len(stableNotes) != 1 {
		t.Fatalf("expected 1 note with stable key, got %d", len(stableNotes))
	}

	final := stableNotes[0]
	if final.ID != note.ID || final.Title != note.Title || final.Body != note.Body {
		t.Fatalf("unexpected note content with stable key: %+v", final)
	}
}

func TestDefaultPathUsesBaseDir(t *testing.T) {
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, "brand")

	path, err := DefaultPath(baseDir)
	if err != nil {
		t.Fatalf("default path: %v", err)
	}

	expected := filepath.Join(baseDir, "notes.json")
	if path != expected {
		t.Fatalf("expected notes path %s, got %s", expected, path)
	}
}

func TestDefaultPathTrimsInput(t *testing.T) {
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, "another")

	path, err := DefaultPath("  " + baseDir + "  ")
	if err != nil {
		t.Fatalf("default path: %v", err)
	}

	expected := filepath.Join(baseDir, "notes.json")
	if path != expected {
		t.Fatalf("expected trimmed notes path %s, got %s", expected, path)
	}
}
