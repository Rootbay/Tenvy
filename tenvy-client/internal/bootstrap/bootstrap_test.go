package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandUsesOverride(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	stubPath := filepath.Join(tempDir, "stub")
	loaderPath := filepath.Join(tempDir, "loader.exe")

	if err := os.WriteFile(stubPath, []byte("stub"), 0o644); err != nil {
		t.Fatalf("failed to write stub file: %v", err)
	}
	if err := os.WriteFile(loaderPath, []byte("loader"), 0o644); err != nil {
		t.Fatalf("failed to write loader file: %v", err)
	}

	ctx := context.Background()
	cmd, err := Command(ctx, Options{
		ExecutablePath: stubPath,
		OverridePath:   loaderPath,
		LoaderArgs:     []string{"--hello", "world"},
		BaseEnv:        []string{"FOO=BAR"},
		AdditionalEnv:  map[string]string{"EXTRA": "1"},
	})
	if err != nil {
		t.Fatalf("Command returned error: %v", err)
	}

	if cmd.Path != loaderPath {
		t.Fatalf("expected loader path %q, got %q", loaderPath, cmd.Path)
	}

	expectedArgs := append([]string{loaderPath}, "--hello", "world")
	if !slicesEqual(cmd.Args, expectedArgs) {
		t.Fatalf("unexpected args: got %v, want %v", cmd.Args, expectedArgs)
	}

	env := envMap(cmd.Env)
	if env["FOO"] != "BAR" {
		t.Fatalf("expected FOO=BAR in environment, got %q", env["FOO"])
	}
	if env["EXTRA"] != "1" {
		t.Fatalf("expected EXTRA=1 in environment, got %q", env["EXTRA"])
	}
	if env["TENVY_LOADER_EXECUTABLE"] != loaderPath {
		t.Fatalf("expected TENVY_LOADER_EXECUTABLE=%q, got %q", loaderPath, env["TENVY_LOADER_EXECUTABLE"])
	}
}

func TestCommandDiscoversLoaderRelativeToStub(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	stubPath := filepath.Join(tempDir, "stub")
	loaderDir := filepath.Join(tempDir, "loader")
	loaderPath := filepath.Join(loaderDir, "tenvy-client-loader")

	if err := os.WriteFile(stubPath, []byte("stub"), 0o644); err != nil {
		t.Fatalf("failed to write stub file: %v", err)
	}
	if err := os.MkdirAll(loaderDir, 0o755); err != nil {
		t.Fatalf("failed to create loader dir: %v", err)
	}
	if err := os.WriteFile(loaderPath, []byte("loader"), 0o644); err != nil {
		t.Fatalf("failed to write loader file: %v", err)
	}

	cmd, err := Command(context.Background(), Options{
		ExecutablePath: stubPath,
	})
	if err != nil {
		t.Fatalf("Command returned error: %v", err)
	}

	if cmd.Path != loaderPath {
		t.Fatalf("expected loader path %q, got %q", loaderPath, cmd.Path)
	}

	if cmd.Dir != loaderDir {
		t.Fatalf("expected command dir %q, got %q", loaderDir, cmd.Dir)
	}
}

func TestCommandAcceptsRelativeOverride(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	stubPath := filepath.Join(tempDir, "stub")
	loaderPath := filepath.Join(tempDir, "bin", "loader.exe")

	if err := os.WriteFile(stubPath, []byte("stub"), 0o644); err != nil {
		t.Fatalf("failed to write stub file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(loaderPath), 0o755); err != nil {
		t.Fatalf("failed to create loader dir: %v", err)
	}
	if err := os.WriteFile(loaderPath, []byte("loader"), 0o644); err != nil {
		t.Fatalf("failed to write loader file: %v", err)
	}

	cmd, err := Command(context.Background(), Options{
		ExecutablePath: stubPath,
		OverridePath:   filepath.Join("bin", "loader.exe"),
	})
	if err != nil {
		t.Fatalf("Command returned error: %v", err)
	}

	if cmd.Path != loaderPath {
		t.Fatalf("expected loader path %q, got %q", loaderPath, cmd.Path)
	}
}

func TestCommandMissingLoader(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	stubPath := filepath.Join(tempDir, "stub")
	if err := os.WriteFile(stubPath, []byte("stub"), 0o644); err != nil {
		t.Fatalf("failed to write stub file: %v", err)
	}

	if _, err := Command(context.Background(), Options{ExecutablePath: stubPath}); err == nil {
		t.Fatalf("expected error when loader missing")
	}
}

func TestBuildSearchDirsIncludesAdditionalPaths(t *testing.T) {
	t.Parallel()

	stubDir := filepath.Join("/tmp", "stub")
	dirs := buildSearchDirs(stubDir, []string{"bin", "/opt/tenvy"})

	expected := []string{
		filepath.Clean(stubDir),
		filepath.Join(filepath.Clean(stubDir), "loader"),
		filepath.Join(filepath.Clean(stubDir), "bin"),
		filepath.Clean("/opt/tenvy"),
	}
	if !slicesEqual(dirs, expected) {
		t.Fatalf("unexpected dirs: got %v, want %v", dirs, expected)
	}
}

func TestEnvironmentDefaultBase(t *testing.T) {
	stubPath := mustWriteTempFile(t)
	loaderPath := mustWriteTempFile(t)

	t.Setenv("TEST_ENV_KEY", "VALUE")
	cmd, err := Command(context.Background(), Options{
		ExecutablePath: stubPath,
		OverridePath:   loaderPath,
		AdditionalEnv:  map[string]string{"CUSTOM": "yes"},
	})
	if err != nil {
		t.Fatalf("Command returned error: %v", err)
	}

	env := envMap(cmd.Env)
	if env["CUSTOM"] != "yes" {
		t.Fatalf("expected CUSTOM env override, got %q", env["CUSTOM"])
	}
	if env["TEST_ENV_KEY"] != "VALUE" {
		t.Fatalf("expected TEST_ENV_KEY inherited, got %q", env["TEST_ENV_KEY"])
	}
}

func mustWriteTempFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func envMap(env []string) map[string]string {
	result := make(map[string]string, len(env))
	for _, entry := range env {
		if entry == "" {
			continue
		}
		key, value, found := strings.Cut(entry, "=")
		if !found {
			result[entry] = ""
			continue
		}
		result[key] = value
	}
	return result
}

func TestEnsureFileRejectsDirectories(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	if err := ensureFile(tempDir, realFileSystem{}); err == nil {
		t.Fatalf("expected directory to be rejected")
	}
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	stubPath := filepath.Join("/opt", "tenvy", "stub")
	rel := filepath.Join("..", "bin", "loader")
	normalized := normalizePath(stubPath, rel)
	expected := filepath.Join("/opt", "tenvy", "..", "bin", "loader")
	if normalized != filepath.Clean(expected) {
		t.Fatalf("unexpected normalized path: got %q, want %q", normalized, filepath.Clean(expected))
	}
}
