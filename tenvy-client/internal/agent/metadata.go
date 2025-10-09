package agent

import (
	"net"
	"os"
	"os/user"
	"runtime"
	"strings"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func CollectMetadata(buildVersion string) protocol.AgentMetadata {
	hostname, _ := os.Hostname()
	currentUser, err := user.Current()
	username := "unknown"
	if err == nil {
		username = currentUser.Username
	} else if val := os.Getenv("USER"); val != "" {
		username = val
	} else if val := os.Getenv("USERNAME"); val != "" {
		username = val
	}

	tags := parseTags(os.Getenv("TENVY_AGENT_TAGS"))

	return protocol.AgentMetadata{
		Hostname:     fallback(hostname, "unknown"),
		Username:     username,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		IPAddress:    detectPrimaryIP(),
		Tags:         tags,
		Version:      buildVersion,
	}
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

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}
