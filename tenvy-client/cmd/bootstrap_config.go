package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type bootstrapConfig struct {
	Controller controllerConfig `json:"controller"`
	Loader     loaderConfig     `json:"loader"`
}

type controllerConfig struct {
	BaseURL string `json:"baseUrl"`
}

type loaderConfig struct {
	Version      string `json:"version"`
	Checksum     string `json:"checksum"`
	Signature    string `json:"signature"`
	Executable   string `json:"executable"`
	ArtifactURL  string `json:"artifactUrl"`
	ArtifactType string `json:"artifactType"`
	Mode         string `json:"mode"`
}

var defaultBootstrapConfigEncoded = ""

func loadBootstrapConfig(logger *log.Logger, stubPath string) (*bootstrapConfig, error) {
	if override := strings.TrimSpace(os.Getenv("TENVY_BOOTSTRAP_CONFIG")); override != "" {
		return readBootstrapConfigFromFile(override)
	}

	if stubPath != "" {
		diskPath := filepath.Join(filepath.Dir(stubPath), "tenvy-bootstrap.json")
		if cfg, err := readBootstrapConfigFromFile(diskPath); err == nil {
			return cfg, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	encoded := strings.TrimSpace(defaultBootstrapConfigEncoded)
	if encoded == "" {
		return nil, errors.New("bootstrap configuration unavailable")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		if logger != nil {
			logger.Printf("embedded bootstrap config invalid: %v", err)
		}
		return nil, errors.New("bootstrap configuration unavailable")
	}
	return parseBootstrapConfig(decoded)
}

func readBootstrapConfigFromFile(path string) (*bootstrapConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := parseBootstrapConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parse bootstrap config %q: %w", path, err)
	}
	return cfg, nil
}

func parseBootstrapConfig(data []byte) (*bootstrapConfig, error) {
	var cfg bootstrapConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode bootstrap config: %w", err)
	}
	cfg.Controller.BaseURL = strings.TrimSpace(cfg.Controller.BaseURL)
	if cfg.Controller.BaseURL == "" {
		return nil, errors.New("controller.baseUrl is required")
	}
	base, err := url.Parse(cfg.Controller.BaseURL)
	if err != nil || !base.IsAbs() || base.Host == "" {
		return nil, fmt.Errorf("controller.baseUrl invalid: %v", err)
	}

	cfg.Loader.Version = strings.TrimSpace(cfg.Loader.Version)
	cfg.Loader.Checksum = strings.TrimSpace(cfg.Loader.Checksum)
	cfg.Loader.Signature = strings.TrimSpace(cfg.Loader.Signature)
	cfg.Loader.Executable = strings.TrimSpace(cfg.Loader.Executable)
	cfg.Loader.ArtifactURL = strings.TrimSpace(cfg.Loader.ArtifactURL)
	cfg.Loader.ArtifactType = strings.ToLower(strings.TrimSpace(cfg.Loader.ArtifactType))
	cfg.Loader.Mode = strings.TrimSpace(cfg.Loader.Mode)

	if cfg.Loader.ArtifactURL == "" {
		return nil, errors.New("loader.artifactUrl is required")
	}
	if _, err := url.Parse(cfg.Loader.ArtifactURL); err != nil {
		return nil, fmt.Errorf("loader.artifactUrl invalid: %v", err)
	}
	return &cfg, nil
}

func (l loaderConfig) resolvedArtifactURL(base string) (string, error) {
	parsedBase, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	artifact, err := url.Parse(l.ArtifactURL)
	if err != nil {
		return "", fmt.Errorf("parse artifact url: %w", err)
	}
	if artifact.IsAbs() {
		if artifact.Scheme == "" || artifact.Host == "" {
			return "", errors.New("loader artifact url missing scheme or host")
		}
		return artifact.String(), nil
	}
	resolved := parsedBase.ResolveReference(artifact)
	return resolved.String(), nil
}

func (l loaderConfig) parsedMode() (os.FileMode, error) {
	trimmed := strings.TrimSpace(l.Mode)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(trimmed, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("parse loader mode: %w", err)
	}
	return os.FileMode(value), nil
}
