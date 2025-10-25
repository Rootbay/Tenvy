package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/rootbay/tenvy-client/internal/bootstrap"
)

var buildVersion = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	logger := log.New(os.Stdout, "[tenvy-bootstrap] ", log.LstdFlags|log.Lmsgprefix)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	stubPath, err := os.Executable()
	if err != nil {
		logger.Printf("resolve executable: %v", err)
		return 1
	}

	stubPath, err = filepath.Abs(stubPath)
	if err != nil {
		logger.Printf("resolve executable path: %v", err)
		return 1
	}

	override := strings.TrimSpace(firstNonEmpty(
		os.Getenv("TENVY_LOADER_PATH"),
		os.Getenv("TENVY_LOADER_EXECUTABLE"),
	))

	searchDirs := parseSearchDirs(os.Getenv("TENVY_LOADER_SEARCH_PATHS"))

	cfg, err := loadBootstrapConfig(logger, stubPath)
	if err != nil {
		logger.Printf("bootstrap config: %v", err)
		return 1
	}

	artifactURL, err := cfg.Loader.resolvedArtifactURL(cfg.Controller.BaseURL)
	if err != nil {
		logger.Printf("resolve loader url: %v", err)
		return 1
	}

	mode, err := cfg.Loader.parsedMode()
	if err != nil {
		logger.Printf("loader mode: %v", err)
		return 1
	}

	artifactType := bootstrap.LoaderArtifactType(strings.ToLower(strings.TrimSpace(cfg.Loader.ArtifactType)))
	if artifactType == "" {
		artifactType = bootstrap.LoaderArtifactTypeBinary
	}

	downloader, err := bootstrap.NewHTTPDownloader(bootstrap.HTTPDownloaderConfig{
		Client:       bootstrap.DefaultHTTPClient(),
		URL:          artifactURL,
		ArtifactType: artifactType,
		Mode:         fs.FileMode(mode),
	})
	if err != nil {
		logger.Printf("configure loader downloader: %v", err)
		return 1
	}

	metadata := &bootstrap.LoaderMetadata{
		Version:    cfg.Loader.Version,
		Checksum:   cfg.Loader.Checksum,
		Signature:  cfg.Loader.Signature,
		Executable: cfg.Loader.Executable,
	}

	opts := bootstrap.Options{
		ExecutablePath: stubPath,
		OverridePath:   override,
		LoaderArgs:     os.Args[1:],
		AdditionalEnv: map[string]string{
			"TENVY_PARENT_PID":          strconv.Itoa(os.Getpid()),
			"TENVY_STUB_EXECUTABLE":     stubPath,
			"TENVY_STUB_DIRECTORY":      filepath.Dir(stubPath),
			"TENVY_STUB_VERSION":        buildVersion,
			"TENVY_CONTROLLER_BASE_URL": cfg.Controller.BaseURL,
			"TENVY_LOADER_VERSION":      cfg.Loader.Version,
		},
		SearchDirs:              searchDirs,
		DesiredLoader:           metadata,
		LoaderDownloader:        downloader,
		LoaderSignatureVerifier: bootstrap.NewLoaderSignatureVerifier(),
	}

	cmd, err := bootstrap.Command(ctx, opts)
	if err != nil {
		logger.Printf("bootstrap error: %v", err)
		return 1
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Printf("capture loader stdout: %v", err)
		return 1
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Printf("capture loader stderr: %v", err)
		return 1
	}

	logger.Printf("starting loader: %s", cmd.Path)
	if err := cmd.Start(); err != nil {
		logger.Printf("failed to start loader: %v", err)
		return 1
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamPipe(&wg, stdout, logger, "stdout")
	go streamPipe(&wg, stderr, logger, "stderr")

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			logger.Printf("loader cancelled: %v", err)
			exitCode = 1
		default:
			var exitErr *os.SyscallError
			if errors.As(err, &exitErr) {
				logger.Printf("loader syscall error: %v", exitErr)
				exitCode = 1
			} else if exitStatus := exitCodeFromError(err); exitStatus >= 0 {
				exitCode = exitStatus
				if exitCode != 0 {
					logger.Printf("loader exited with code %d", exitCode)
				}
			} else {
				logger.Printf("loader execution error: %v", err)
				exitCode = 1
			}
		}
	} else if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	wg.Wait()

	if exitCode == 0 {
		logger.Printf("loader exited successfully")
	}

	return exitCode
}

func streamPipe(wg *sync.WaitGroup, reader io.Reader, logger *log.Logger, name string) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		logger.Printf("%s: %s", name, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		logger.Printf("%s stream error: %v", name, err)
	}
}

func parseSearchDirs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := filepath.SplitList(raw)
	dirs := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			dirs = append(dirs, trimmed)
		}
	}
	return dirs
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func exitCodeFromError(err error) int {
	type exitCoder interface {
		ExitCode() int
	}
	var coder exitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	return -1
}
