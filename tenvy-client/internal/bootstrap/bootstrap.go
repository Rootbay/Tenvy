package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// FileSystem abstracts stat calls so loader discovery can be tested without touching the real disk.
type FileSystem interface {
	Stat(name string) (fs.FileInfo, error)
}

type realFileSystem struct{}

func (realFileSystem) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// Options control how the loader command is constructed.
type Options struct {
	// ExecutablePath is the absolute path to the stub executable. It is required.
	ExecutablePath string
	// OverridePath explicitly points to the loader executable, bypassing discovery when provided.
	OverridePath string
	// LoaderArgs are forwarded to the loader invocation.
	LoaderArgs []string
	// BaseEnv is the environment inherited by the loader. Defaults to os.Environ when nil.
	BaseEnv []string
	// AdditionalEnv defines extra environment variables to inject for the loader.
	AdditionalEnv map[string]string
	// SearchDirs are additional directories (absolute or relative to ExecutablePath) inspected for the loader.
	SearchDirs []string
	// CandidateNames are filenames considered when looking for the loader.
	CandidateNames []string
	// FileSystem powers file discovery. Defaults to the real OS filesystem when nil.
	FileSystem FileSystem
}

// Command builds an exec.Cmd ready to launch the loader process based on the provided options.
func Command(ctx context.Context, opts Options) (*exec.Cmd, error) {
	if strings.TrimSpace(opts.ExecutablePath) == "" {
		return nil, errors.New("executable path is required")
	}

	fsys := opts.FileSystem
	if fsys == nil {
		fsys = realFileSystem{}
	}

	loaderPath, err := discoverLoader(opts, fsys)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, loaderPath, opts.LoaderArgs...)
	cmd.Env = buildEnvironment(opts, loaderPath)
	cmd.Dir = filepath.Dir(loaderPath)
	cmd.Stdin = os.Stdin
	return cmd, nil
}

func discoverLoader(opts Options, fsys FileSystem) (string, error) {
	override := strings.TrimSpace(opts.OverridePath)
	if override != "" {
		path := normalizePath(opts.ExecutablePath, override)
		if err := ensureFile(path, fsys); err != nil {
			return "", fmt.Errorf("loader override %q invalid: %w", override, err)
		}
		return path, nil
	}

	stubDir := filepath.Dir(opts.ExecutablePath)
	searchDirs := buildSearchDirs(stubDir, opts.SearchDirs)
	candidateNames := opts.CandidateNames
	if len(candidateNames) == 0 {
		candidateNames = defaultCandidateNames()
	}

	visited := make(map[string]struct{})
	for _, dir := range searchDirs {
		for _, name := range candidateNames {
			candidate := name
			if !filepath.IsAbs(candidate) {
				candidate = filepath.Join(dir, candidate)
			}
			cleaned := filepath.Clean(candidate)
			if _, seen := visited[cleaned]; seen {
				continue
			}
			visited[cleaned] = struct{}{}

			if err := ensureFile(cleaned, fsys); err == nil {
				return cleaned, nil
			}
		}
	}

	return "", errors.New("loader executable not found")
}

func buildEnvironment(opts Options, loaderPath string) []string {
	base := opts.BaseEnv
	if base == nil {
		base = os.Environ()
	}

	merged := make(map[string]string, len(base)+len(opts.AdditionalEnv)+1)
	for _, entry := range base {
		if entry == "" {
			continue
		}
		key, value, found := strings.Cut(entry, "=")
		if !found {
			merged[key] = ""
			continue
		}
		merged[key] = value
	}

	for key, value := range opts.AdditionalEnv {
		merged[key] = value
	}

	merged["TENVY_LOADER_EXECUTABLE"] = loaderPath

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+merged[key])
	}
	return env
}

func ensureFile(path string, fsys FileSystem) error {
	if path == "" {
		return errors.New("empty path")
	}
	info, err := fsys.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}

func buildSearchDirs(stubDir string, extra []string) []string {
	dirs := make([]string, 0, len(extra)+3)
	seen := make(map[string]struct{})
	add := func(path string) {
		cleaned := filepath.Clean(path)
		if _, exists := seen[cleaned]; exists {
			return
		}
		seen[cleaned] = struct{}{}
		dirs = append(dirs, cleaned)
	}

	if stubDir != "" {
		add(stubDir)
		add(filepath.Join(stubDir, "loader"))
		add(filepath.Join(stubDir, "bin"))
	}

	for _, dir := range extra {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		candidate := dir
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(stubDir, candidate)
		}
		add(candidate)
	}

	return dirs
}

func defaultCandidateNames() []string {
	return []string{
		"tenvy-client-loader",
		"tenvy-client-loader.exe",
		"loader",
		"loader.exe",
	}
}

func normalizePath(stubExecutable, path string) string {
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return cleaned
	}
	return filepath.Join(filepath.Dir(stubExecutable), cleaned)
}
