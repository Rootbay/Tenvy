package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type stubClock struct {
	now time.Time
}

func (c *stubClock) Now() time.Time {
	if c.now.IsZero() {
		c.now = time.Date(2024, 6, 1, 9, 30, 0, 0, time.UTC)
	}
	return c.now
}

type commandResponse struct {
	Action string       `json:"action"`
	Status string       `json:"status"`
	Result statusResult `json:"result"`
}

type fakeProcessEnumerator struct {
	samples []processSample
}

func (f *fakeProcessEnumerator) Processes(context.Context) ([]processSample, error) {
	if f.samples == nil {
		return []processSample{}, nil
	}
	out := make([]processSample, len(f.samples))
	copy(out, f.samples)
	return out, nil
}

type fakeConnectionEnumerator struct {
	samples []connectionSample
}

func (f *fakeConnectionEnumerator) Connections(context.Context) ([]connectionSample, error) {
	if f.samples == nil {
		return []connectionSample{}, nil
	}
	out := make([]connectionSample, len(f.samples))
	copy(out, f.samples)
	return out, nil
}

type fakeResolver struct {
	records map[string][]string
	err     error
}

func (f *fakeResolver) LookupIP(_ context.Context, host string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.records) == 0 {
		return nil, fmt.Errorf("no records")
	}
	ips, ok := f.records[strings.ToLower(host)]
	if !ok {
		return nil, fmt.Errorf("no records")
	}
	out := make([]string, len(ips))
	copy(out, ips)
	return out, nil
}

func decodeResponse(t *testing.T, data string) commandResponse {
	t.Helper()
	if data == "" {
		t.Fatal("empty command output")
	}
	var decoded commandResponse
	if err := json.Unmarshal([]byte(data), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return decoded
}

func TestManagerReportsProcessActivity(t *testing.T) {
	manager := NewManager()
	manager.setClock(&stubClock{})

	procEnumerator := &fakeProcessEnumerator{}
	manager.setProcessEnumerator(procEnumerator)
	manager.setConnectionEnumerator(&fakeConnectionEnumerator{})
	manager.setResolver(&fakeResolver{records: map[string][]string{}})

	procEnumerator.samples = []processSample{{
		PID:         4321,
		Name:        "calc.exe",
		Executable:  `C:\\Windows\\System32\\calc.exe`,
		CommandLine: `"C:\\Windows\\System32\\calc.exe"`,
	}}

	payload, err := json.Marshal(commandPayload{
		Action: "configure",
		Config: monitorCommand{
			Feed:               "live",
			RefreshSeconds:     2,
			IncludeScreenshots: false,
			IncludeCommands:    true,
			Watchlist: []watchEntry{{
				Kind:         "app",
				ID:           "calc.exe",
				DisplayName:  "Calculator",
				AlertOnOpen:  true,
				AlertOnClose: true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "configure", Payload: payload})
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	decoded := decodeResponse(t, result.Output)
	if decoded.Action != "configure" {
		t.Fatalf("expected configure action, got %s", decoded.Action)
	}
	if decoded.Status != "ok" {
		t.Fatalf("expected ok status, got %s", decoded.Status)
	}

	if len(decoded.Result.Events) == 0 {
		t.Fatalf("expected at least one event")
	}
	if decoded.Result.Events[0].Event != "open" {
		t.Fatalf("expected open event, got %s", decoded.Result.Events[0].Event)
	}

	var processMetric *metric
	for i := range decoded.Result.Metrics {
		metric := decoded.Result.Metrics[i]
		if metric.ID == "app:calc.exe" {
			processMetric = &metric
			break
		}
	}
	if processMetric == nil {
		t.Fatalf("expected process metric for calc.exe")
	}
	if !strings.Contains(processMetric.Value, "Active: 1") {
		t.Fatalf("expected metric to show active process, got %q", processMetric.Value)
	}
}

func TestManagerEmitsCloseEventWhenProcessStops(t *testing.T) {
	manager := NewManager()
	clock := &stubClock{}
	manager.setClock(clock)

	procEnumerator := &fakeProcessEnumerator{}
	manager.setProcessEnumerator(procEnumerator)
	manager.setConnectionEnumerator(&fakeConnectionEnumerator{})
	manager.setResolver(&fakeResolver{records: map[string][]string{}})

	watchlist := []watchEntry{{
		Kind:         "app",
		ID:           "demo.exe",
		DisplayName:  "Demo",
		AlertOnOpen:  true,
		AlertOnClose: true,
	}}

	procEnumerator.samples = []processSample{{
		PID:         101,
		Name:        "demo.exe",
		Executable:  `C:\\Program Files\\Demo\\demo.exe`,
		CommandLine: `"C:\\Program Files\\Demo\\demo.exe"`,
	}}

	configurePayload, err := json.Marshal(commandPayload{
		Action: "configure",
		Config: monitorCommand{Feed: "live", RefreshSeconds: 2, Watchlist: watchlist},
	})
	if err != nil {
		t.Fatalf("marshal configure payload: %v", err)
	}
	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "configure", Payload: configurePayload})
	if !result.Success {
		t.Fatalf("configure command failed: %s", result.Error)
	}

	procEnumerator.samples = []processSample{}
	clock.now = clock.now.Add(10 * time.Second)

	status := manager.HandleCommand(context.Background(), protocol.Command{ID: "status"})
	if !status.Success {
		t.Fatalf("status command failed: %s", status.Error)
	}

	decoded := decodeResponse(t, status.Output)
	if len(decoded.Result.Events) < 2 {
		t.Fatalf("expected open and close events, got %d", len(decoded.Result.Events))
	}
	if decoded.Result.Events[0].Event != "close" {
		t.Fatalf("expected latest event to be close, got %s", decoded.Result.Events[0].Event)
	}
	if decoded.Result.Events[1].Event != "open" {
		t.Fatalf("expected prior event to be open, got %s", decoded.Result.Events[1].Event)
	}
}

func TestManagerTracksURLConnections(t *testing.T) {
	manager := NewManager()
	clock := &stubClock{}
	manager.setClock(clock)

	procEnumerator := &fakeProcessEnumerator{}
	connEnumerator := &fakeConnectionEnumerator{}
	resolver := &fakeResolver{records: map[string][]string{
		"example.com": {"203.0.113.10"},
	}}

	manager.setProcessEnumerator(procEnumerator)
	manager.setConnectionEnumerator(connEnumerator)
	manager.setResolver(resolver)

	watchlist := []watchEntry{{
		Kind:         "url",
		ID:           "https://example.com/dashboard",
		DisplayName:  "Example",
		AlertOnOpen:  true,
		AlertOnClose: true,
	}}

	connEnumerator.samples = []connectionSample{{
		PID:        808,
		RemoteIP:   "203.0.113.10",
		RemotePort: 443,
		Status:     "ESTABLISHED",
	}}

	configurePayload, err := json.Marshal(commandPayload{
		Action: "configure",
		Config: monitorCommand{Feed: "live", RefreshSeconds: 2, Watchlist: watchlist},
	})
	if err != nil {
		t.Fatalf("marshal configure payload: %v", err)
	}

	result := manager.HandleCommand(context.Background(), protocol.Command{ID: "configure", Payload: configurePayload})
	if !result.Success {
		t.Fatalf("configure failed: %s", result.Error)
	}

	decoded := decodeResponse(t, result.Output)
	if len(decoded.Result.Events) == 0 {
		t.Fatalf("expected url open event")
	}
	if decoded.Result.Events[0].Event != "open" {
		t.Fatalf("expected open event, got %s", decoded.Result.Events[0].Event)
	}

	connEnumerator.samples = []connectionSample{}
	clock.now = clock.now.Add(5 * time.Second)

	status := manager.HandleCommand(context.Background(), protocol.Command{ID: "status"})
	if !status.Success {
		t.Fatalf("status command failed: %s", status.Error)
	}

	followup := decodeResponse(t, status.Output)
	if len(followup.Result.Events) < 2 {
		t.Fatalf("expected open and close events, got %d", len(followup.Result.Events))
	}
	if followup.Result.Events[0].Event != "close" {
		t.Fatalf("expected close event, got %s", followup.Result.Events[0].Event)
	}
}
