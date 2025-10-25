package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// HTTPDownloaderConfig configures the HTTP-based loader downloader.
type HTTPDownloaderConfig struct {
	// Client is the HTTP client used for requests. When nil, http.DefaultClient is used.
	Client *http.Client
	// URL is the absolute URL of the loader artifact.
	URL string
	// ArtifactType indicates whether the remote payload is a zip archive or raw binary.
	ArtifactType LoaderArtifactType
	// Mode specifies the filesystem mode to apply to binary payloads. A zero value
	// defers to the installer defaults.
	Mode fs.FileMode
}

// LoaderArtifactType enumerates supported artifact encodings.
type LoaderArtifactType string

const (
	// LoaderArtifactTypeBinary indicates the remote payload is a single executable.
	LoaderArtifactTypeBinary LoaderArtifactType = "binary"
	// LoaderArtifactTypeArchive indicates the remote payload is a zip archive containing the loader files.
	LoaderArtifactTypeArchive LoaderArtifactType = "zip"
)

// NewHTTPDownloader constructs a LoaderDownloader implementation that retrieves the loader
// artifact over HTTP based on the provided configuration.
func NewHTTPDownloader(cfg HTTPDownloaderConfig) (LoaderDownloader, error) {
	trimmedURL := strings.TrimSpace(cfg.URL)
	if trimmedURL == "" {
		return nil, errors.New("loader downloader requires url")
	}
	artifactType := cfg.ArtifactType
	if artifactType == "" {
		artifactType = LoaderArtifactTypeBinary
	}
	switch artifactType {
	case LoaderArtifactTypeBinary, LoaderArtifactTypeArchive:
		// supported
	default:
		return nil, fmt.Errorf("unsupported loader artifact type: %s", artifactType)
	}
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}
	return LoaderDownloaderFunc(func(ctx context.Context, metadata LoaderMetadata) (LoaderPackage, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimmedURL, nil)
		if err != nil {
			return LoaderPackage{}, fmt.Errorf("build loader request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return LoaderPackage{}, fmt.Errorf("fetch loader: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return LoaderPackage{}, fmt.Errorf("fetch loader: unexpected status %d", resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return LoaderPackage{}, fmt.Errorf("read loader: %w", err)
		}
		pkg := LoaderPackage{}
		switch artifactType {
		case LoaderArtifactTypeArchive:
			pkg.Archive = data
		case LoaderArtifactTypeBinary:
			pkg.Binary = data
			pkg.Mode = cfg.Mode
		}
		return pkg, nil
	}), nil
}

// DefaultHTTPClient returns an HTTP client tuned for loader downloads.
func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}
