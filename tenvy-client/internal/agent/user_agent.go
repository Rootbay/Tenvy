package agent

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

const (
	userAgentFingerprintEdgeWindows  = "edge_win10"
	userAgentFingerprintChromeMac    = "chrome_macos"
	userAgentFingerprintFirefoxLinux = "firefox_linux"
)

type userAgentMetadata struct {
	OS     string
	Arch   string
	Locale string
}

type userAgentFingerprint struct {
	build func(userAgentMetadata) string
}

var userAgentFingerprintRegistry = map[string]userAgentFingerprint{
	userAgentFingerprintEdgeWindows:  {build: buildEdgeWindowsUserAgent},
	userAgentFingerprintChromeMac:    {build: buildChromeMacUserAgent},
	userAgentFingerprintFirefoxLinux: {build: buildFirefoxLinuxUserAgent},
}

func resolveUserAgentString(override, fingerprint string, disable bool, fallbackVersion string) string {
	meta := currentUserAgentMetadata()
	return resolveUserAgentWithMetadata(override, fingerprint, disable, fallbackVersion, meta)
}

func resolveUserAgentWithMetadata(override, fingerprint string, disable bool, fallbackVersion string, meta userAgentMetadata) string {
	if ua := strings.TrimSpace(override); ua != "" {
		return ua
	}

	if ua := generateUserAgentFromFingerprint(fingerprint, meta); ua != "" {
		return ua
	}

	if disable {
		return ""
	}

	if ua := generateUserAgentFromFingerprint(defaultUserAgentFingerprint(), meta); ua != "" {
		return ua
	}

	version := strings.TrimSpace(fallbackVersion)
	if version == "" {
		version = "unknown"
	}
	return fmt.Sprintf("tenvy-client/%s", version)
}

func currentUserAgentMetadata() userAgentMetadata {
	return userAgentMetadata{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Locale: detectPreferredLocale(),
	}
}

func detectPreferredLocale() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		if idx := strings.Index(value, "."); idx >= 0 {
			value = value[:idx]
		}
		if idx := strings.Index(value, "@"); idx >= 0 {
			value = value[:idx]
		}
		value = strings.ReplaceAll(value, "_", "-")
		if parts := strings.SplitN(value, "-", 2); len(parts) == 2 {
			return strings.ToLower(parts[0]) + "-" + strings.ToUpper(parts[1])
		}
		if len(value) == 2 {
			return strings.ToLower(value) + "-" + strings.ToUpper(value)
		}
		if value != "" {
			return value
		}
	}
	return "en-US"
}

func generateUserAgentFromFingerprint(name string, meta userAgentMetadata) string {
	normalized := normalizeFingerprintName(name)
	if normalized == "" {
		return ""
	}
	fingerprint, ok := userAgentFingerprintRegistry[normalized]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fingerprint.build(meta))
}

func normalizeFingerprintName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	lowered := strings.ToLower(trimmed)
	lowered = strings.ReplaceAll(lowered, " ", "_")
	lowered = strings.ReplaceAll(lowered, "-", "_")
	return lowered
}

func defaultUserAgentFingerprint() string {
	return defaultFingerprintForRuntime(runtime.GOOS)
}

func defaultFingerprintForRuntime(goos string) string {
	switch goos {
	case "windows":
		return userAgentFingerprintEdgeWindows
	case "darwin":
		return userAgentFingerprintChromeMac
	default:
		return userAgentFingerprintFirefoxLinux
	}
}

func buildEdgeWindowsUserAgent(meta userAgentMetadata) string {
	archToken := "Win64; x64"
	switch meta.Arch {
	case "386":
		archToken = "Win32; x86"
	case "arm64":
		archToken = "Win64; ARM64"
	}
	segments := []string{"Windows NT 10.0"}
	if archToken != "" {
		segments = append(segments, archToken)
	}
	if loc := strings.TrimSpace(meta.Locale); loc != "" {
		segments = append(segments, loc)
	}
	platform := strings.Join(segments, "; ")
	return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0", platform)
}

func buildChromeMacUserAgent(meta userAgentMetadata) string {
	platform := "Macintosh; Intel Mac OS X 10_15_7"
	if meta.Arch == "arm64" {
		platform = "Macintosh; Apple Silicon Mac OS X 14_2"
	}
	return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36", platform)
}

func buildFirefoxLinuxUserAgent(meta userAgentMetadata) string {
	archToken := "x86_64"
	switch meta.Arch {
	case "386":
		archToken = "i686"
	case "arm64":
		archToken = "aarch64"
	}
	localeSegment := ""
	if loc := strings.TrimSpace(meta.Locale); loc != "" {
		localeSegment = "; " + loc
	}
	return fmt.Sprintf("Mozilla/5.0 (X11; Linux %s; rv:118.0) Gecko/20100101 Firefox/118.0%s", archToken, localeSegment)
}
