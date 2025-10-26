package geolocation

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type (
	Command       = protocol.Command
	CommandResult = protocol.CommandResult
)

type clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// Manager executes geolocation lookups using synthetic provider data.
type Manager struct {
	mu              sync.Mutex
	clock           clock
	providers       []string
	defaultProvider string
	last            *lookupResult
}

type commandPayload struct {
	Action          string `json:"action"`
	IP              string `json:"ip,omitempty"`
	Provider        string `json:"provider,omitempty"`
	IncludeTimezone bool   `json:"includeTimezone,omitempty"`
	IncludeMap      bool   `json:"includeMap,omitempty"`
}

type lookupResult struct {
	IP          string    `json:"ip"`
	Provider    string    `json:"provider"`
	City        string    `json:"city,omitempty"`
	Region      string    `json:"region,omitempty"`
	Country     string    `json:"country"`
	CountryCode string    `json:"countryCode,omitempty"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	NetworkType string    `json:"networkType"`
	ISP         string    `json:"isp,omitempty"`
	ASN         string    `json:"asn,omitempty"`
	Timezone    *timezone `json:"timezone,omitempty"`
	MapURL      string    `json:"mapUrl,omitempty"`
	RetrievedAt string    `json:"retrievedAt"`
}

type timezone struct {
	ID           string `json:"id"`
	Offset       string `json:"offset"`
	Abbreviation string `json:"abbreviation,omitempty"`
}

type statusResult struct {
	LastLookup      *lookupResult `json:"lastLookup"`
	Providers       []string      `json:"providers"`
	DefaultProvider string        `json:"defaultProvider"`
	GeneratedAt     string        `json:"generatedAt"`
}

func NewManager() *Manager {
	return &Manager{
		clock:           systemClock{},
		providers:       []string{"ipinfo", "maxmind", "db-ip"},
		defaultProvider: "ipinfo",
	}
}

func (m *Manager) HandleCommand(ctx context.Context, cmd Command) CommandResult {
	completedAt := time.Now().UTC().Format(time.RFC3339Nano)
	result := CommandResult{CommandID: cmd.ID, CompletedAt: completedAt}

	var payload commandPayload
	if len(cmd.Payload) > 0 {
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("invalid geolocation payload: %v", err)
			return result
		}
	}

	action := strings.TrimSpace(strings.ToLower(payload.Action))
	if action == "" {
		action = "status"
	}

	switch action {
	case "status":
		if err := m.writeStatus(&result); err != nil {
			result.Success = false
			result.Error = err.Error()
			return result
		}
	case "lookup":
		if err := m.performLookup(payload, &result); err != nil {
			result.Success = false
			result.Error = err.Error()
			return result
		}
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unsupported geolocation action: %s", payload.Action)
		return result
	}

	result.Success = true
	return result
}

func (m *Manager) writeStatus(result *CommandResult) error {
	status := statusResult{
		LastLookup:      m.lastLookup(),
		Providers:       append([]string(nil), m.providers...),
		DefaultProvider: m.defaultProvider,
		GeneratedAt:     m.now().Format(time.RFC3339),
	}

	payload, err := json.Marshal(map[string]any{
		"action": "status",
		"status": "ok",
		"result": status,
	})
	if err != nil {
		return err
	}
	result.Output = string(payload)
	return nil
}

func (m *Manager) performLookup(payload commandPayload, result *CommandResult) error {
	ip := strings.TrimSpace(payload.IP)
	if ip == "" {
		return fmt.Errorf("ip address is required")
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid ip address")
	}

	provider := strings.ToLower(strings.TrimSpace(payload.Provider))
	if provider == "" {
		provider = m.defaultProvider
	}
	if !m.isProviderSupported(provider) {
		return fmt.Errorf("unsupported provider: %s", payload.Provider)
	}

	location := synthesizeLocation(parsed)
	location.Provider = provider
	location.IP = ip
	location.NetworkType = classifyNetwork(parsed)
	if payload.IncludeTimezone {
		if location.Timezone != nil {
			location.Timezone = &timezone{ID: location.Timezone.ID, Offset: location.Timezone.Offset, Abbreviation: location.Timezone.Abbreviation}
		}
	} else {
		location.Timezone = nil
	}
	if payload.IncludeMap {
		location.MapURL = fmt.Sprintf("https://maps.example.com/?lat=%.4f&lon=%.4f", location.Latitude, location.Longitude)
	}
	location.RetrievedAt = m.now().Format(time.RFC3339)

	m.storeLookup(&location)

	payloadBytes, err := json.Marshal(map[string]any{
		"action": "lookup",
		"status": "ok",
		"result": location,
	})
	if err != nil {
		return err
	}
	result.Output = string(payloadBytes)
	return nil
}

func (m *Manager) isProviderSupported(provider string) bool {
	for _, candidate := range m.providers {
		if provider == candidate {
			return true
		}
	}
	return false
}

func (m *Manager) storeLookup(result *lookupResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *result
	m.last = &clone
}

func (m *Manager) lastLookup() *lookupResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.last == nil {
		return nil
	}
	clone := *m.last
	return &clone
}

func (m *Manager) now() time.Time {
	if m.clock == nil {
		m.clock = systemClock{}
	}
	return m.clock.Now()
}

func (m *Manager) setClock(c clock) {
	if c == nil {
		m.clock = systemClock{}
		return
	}
	m.clock = c
}

// synthesizeLocation returns deterministic location details for an IP.
func synthesizeLocation(ip net.IP) lookupResult {
	locations := []lookupResult{
		{City: "Lisbon", Region: "Lisboa", Country: "Portugal", CountryCode: "PT", Latitude: 38.7223, Longitude: -9.1393, ISP: "IberNet", ASN: "AS64500", Timezone: &timezone{ID: "Europe/Lisbon", Offset: "+01:00", Abbreviation: "WET"}},
		{City: "Berlin", Region: "Berlin", Country: "Germany", CountryCode: "DE", Latitude: 52.5200, Longitude: 13.4050, ISP: "TeutoCom", ASN: "AS64510", Timezone: &timezone{ID: "Europe/Berlin", Offset: "+01:00", Abbreviation: "CET"}},
		{City: "Toronto", Region: "Ontario", Country: "Canada", CountryCode: "CA", Latitude: 43.6518, Longitude: -79.3832, ISP: "NorthFiber", ASN: "AS64520", Timezone: &timezone{ID: "America/Toronto", Offset: "-05:00", Abbreviation: "EST"}},
		{City: "Singapore", Region: "Central", Country: "Singapore", CountryCode: "SG", Latitude: 1.3521, Longitude: 103.8198, ISP: "LionNet", ASN: "AS64530", Timezone: &timezone{ID: "Asia/Singapore", Offset: "+08:00", Abbreviation: "SGT"}},
	}

	sum := 0
	for _, b := range ip {
		sum += int(b)
	}
	index := sum % len(locations)
	return locations[index]
}

func classifyNetwork(ip net.IP) string {
	if ip.IsLoopback() {
		return "loopback"
	}
	if ip.IsPrivate() {
		return "private"
	}
	if ip.IsMulticast() {
		return "multicast"
	}
	return "public"
}
