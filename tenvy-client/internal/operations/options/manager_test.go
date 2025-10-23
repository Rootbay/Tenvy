package options_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	options "github.com/rootbay/tenvy-client/internal/operations/options"
)

func TestManagerApplyOperationStagesScript(t *testing.T) {
	dir := t.TempDir()
	content := []byte("Write-Host 'hello'")

	manager := options.NewManager(options.ManagerOptions{ScriptDirectory: dir})

	summary, err := manager.ApplyOperation(
		context.Background(),
		"script-file",
		map[string]any{
			"fileName":     "ignored.ps1",
			"size":         int64(len(content)),
			"type":         "text/plain",
			"stagingToken": "stage-token",
		},
		func(ctx context.Context, token string) (*options.ScriptPayload, error) {
			if token != "stage-token" {
				t.Fatalf("unexpected token: %s", token)
			}
			return &options.ScriptPayload{
				Data: content,
				Name: "script.ps1",
				Type: "text/x-powershell",
			}, nil
		},
	)

	if err != nil {
		t.Fatalf("ApplyOperation returned error: %v", err)
	}
	if summary != "Script script.ps1 staged" {
		t.Fatalf("unexpected summary: %s", summary)
	}

	state := manager.Snapshot()
	if state.Script.File == nil {
		t.Fatalf("script file state missing")
	}
	if state.Script.File.Name != "script.ps1" {
		t.Fatalf("unexpected script name: %s", state.Script.File.Name)
	}
	if state.Script.File.Type != "text/x-powershell" {
		t.Fatalf("unexpected script type: %s", state.Script.File.Type)
	}
	if state.Script.File.Path == "" {
		t.Fatalf("script path not persisted")
	}
	if state.Script.File.Checksum == "" {
		t.Fatalf("script checksum not recorded")
	}
	if state.Script.File.Size != int64(len(content)) {
		t.Fatalf("unexpected script size: %d", state.Script.File.Size)
	}

	data, err := os.ReadFile(state.Script.File.Path)
	if err != nil {
		t.Fatalf("failed to read staged script: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("script contents do not match")
	}
	if !strings.Contains(filepath.Base(state.Script.File.Path), "script") {
		t.Fatalf("sanitized filename missing expected component: %s", state.Script.File.Path)
	}
}

func TestManagerApplyOperationRequiresFetcherForToken(t *testing.T) {
	manager := options.NewManager(options.ManagerOptions{ScriptDirectory: t.TempDir()})
	_, err := manager.ApplyOperation(
		context.Background(),
		"script-file",
		map[string]any{
			"fileName":     "script.ps1",
			"size":         int64(32),
			"stagingToken": "missing",
		},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "fetcher unavailable") {
		t.Fatalf("expected fetcher error, got %v", err)
	}
}
