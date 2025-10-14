package agent

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func CollectMetadata(buildVersion string) protocol.AgentMetadata {
	hostname, _ := os.Hostname()
	currentUser, err := user.Current()
	username := resolveUsername(currentUser, err)

	tags := parseTags(os.Getenv("TENVY_AGENT_TAGS"))

	return protocol.AgentMetadata{
		Hostname:        fallback(hostname, "unknown"),
		Username:        username,
		OS:              runtime.GOOS,
		Architecture:    runtime.GOARCH,
		IPAddress:       detectPrimaryIP(),
		PublicIPAddress: detectPublicIP(),
		Tags:            tags,
		Version:         buildVersion,
	}
}

func resolveUsername(currentUser *user.User, err error) string {
	// Prefer environment variables to match typical Windows %USERNAME% value.
	if val := os.Getenv("USERNAME"); val != "" {
		return normalizeUsername(val)
	}
	if val := os.Getenv("USER"); val != "" {
		return normalizeUsername(val)
	}
	if err == nil && currentUser != nil {
		return normalizeUsername(currentUser.Username)
	}
	return "unknown"
}

func normalizeUsername(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	parts := strings.Split(trimmed, "/")
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return "unknown"
	}
	return last
}

func detectPrimaryIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := extractIP(addr)
			if ip == "" {
				continue
			}
			return ip
		}
	}
	return ""
}

func extractIP(addr net.Addr) string {
	switch v := addr.(type) {
	case *net.IPNet:
		if v.IP.IsLoopback() {
			return ""
		}
		ip := v.IP.To4()
		if ip != nil {
			return ip.String()
		}
		if v.IP.To16() != nil {
			return v.IP.String()
		}
	case *net.IPAddr:
		if v.IP.IsLoopback() {
			return ""
		}
		ip := v.IP.To4()
		if ip != nil {
			return ip.String()
		}
		if v.IP.To16() != nil {
			return v.IP.String()
		}
	}
	return ""
}

func parseTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}

func detectPublicIP() string {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Get("https://api.ipify.org?format=text")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return ""
	}
	return ip
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}
