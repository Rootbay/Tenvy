package plugins

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func buildTestPlugin(t *testing.T, source string) string {
	t.Helper()

	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write plugin source: %v", err)
	}

	binary := filepath.Join(tempDir, "plugin")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binary, srcPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		t.Fatalf("build plugin: %v", err)
	}

	return binary
}

func TestLaunchRuntimeStartsProcess(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started.txt")
	source := "package main\nimport (\n\t\"os\"\n\t\"time\"\n)\nfunc main() {\n\tmarker := os.Getenv(\"PLUGIN_TEST_MARKER\")\n\tos.WriteFile(marker, []byte(\"started\"), 0o644)\n\tfor {\n\t\ttime.Sleep(10 * time.Millisecond)\n\t}\n}\n"
	binary := buildTestPlugin(t, source)

	logger := log.New(io.Discard, "", 0)
	t.Setenv("PLUGIN_TEST_MARKER", marker)

	handle, err := LaunchRuntime(context.Background(), binary, RuntimeOptions{
		Name:   "test-plugin",
		Logger: logger,
	})
	if err != nil {
		t.Fatalf("launch runtime: %v", err)
	}

	t.Cleanup(func() {
		_ = handle.Shutdown(context.Background())
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("plugin did not create marker file %s", marker)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := handle.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown runtime: %v", err)
	}

	rh, ok := handle.(*processRuntimeHandle)
	if !ok {
		t.Fatalf("unexpected handle type %T", handle)
	}
	if rh.cmd != nil {
		t.Fatalf("expected command cleared after shutdown, got %#v", rh.cmd)
	}
	if rh.waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(rh.waitErr, &exitErr) {
			t.Fatalf("unexpected wait error: %v", rh.waitErr)
		}
	}
}

func TestLaunchRuntimeMissingEntry(t *testing.T) {
	t.Parallel()

	_, err := LaunchRuntime(context.Background(), filepath.Join(t.TempDir(), "missing"), RuntimeOptions{})
	if err == nil {
		t.Fatal("expected error launching missing entry")
	}
}
