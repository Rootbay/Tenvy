package startup

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeProvider struct {
	listReq    *ListRequest
	listResp   Inventory
	toggleReq  *ToggleRequest
	toggleResp Entry
	createReq  *CreateRequest
	createResp Entry
	removeReq  *RemoveRequest
	removeResp RemoveResult
	err        error
}

const sampleLocation = "HKLM:Software\\Microsoft\\Windows\\CurrentVersion\\Run"

func (f *fakeProvider) List(ctx context.Context, req ListRequest) (Inventory, error) {
	f.listReq = &req
	return f.listResp, f.err
}

func (f *fakeProvider) Toggle(ctx context.Context, req ToggleRequest) (Entry, error) {
	f.toggleReq = &req
	return f.toggleResp, f.err
}

func (f *fakeProvider) Create(ctx context.Context, req CreateRequest) (Entry, error) {
	f.createReq = &req
	return f.createResp, f.err
}

func (f *fakeProvider) Remove(ctx context.Context, req RemoveRequest) (RemoveResult, error) {
	f.removeReq = &req
	return f.removeResp, f.err
}

func marshalPayload(t *testing.T, payload CommandPayload) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

func decodeResponse(t *testing.T, result CommandResult) CommandResponse {
	t.Helper()
	if !result.Success {
		t.Fatalf("expected successful result, got error %q", result.Error)
	}
	var response CommandResponse
	if err := json.Unmarshal([]byte(result.Output), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func TestManagerDispatchesList(t *testing.T) {
	provider := &fakeProvider{
                listResp: Inventory{
                        Entries:     []Entry{{ID: "registry:machine:ZW50cnk=", Name: "Test", Path: "C:/app.exe", Enabled: true, Scope: ScopeMachine, Source: SourceRegistry, Impact: ImpactLow, Location: sampleLocation, LastEvaluatedAt: time.Now().UTC().Format(time.RFC3339Nano)}},
                        GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
                },
        }
	manager := NewManager(nil)
	manager.SetProvider(provider)

	payload := CommandPayload{Request: CommandRequest{Operation: "list"}}
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-list", Payload: marshalPayload(t, payload)})
	response := decodeResponse(t, result)
	if response.Operation != "list" {
		t.Fatalf("unexpected operation %q", response.Operation)
	}
	if provider.listReq == nil {
		t.Fatalf("expected provider to receive list request")
	}
}

func TestManagerCreatesEntries(t *testing.T) {
	provider := &fakeProvider{
                createResp: Entry{ID: "registry:machine:dGVzdA==", Name: "test", Path: "C:/tool.exe", Enabled: true, Scope: ScopeMachine, Source: SourceRegistry, Impact: ImpactNotMeasured, Location: sampleLocation, LastEvaluatedAt: time.Now().UTC().Format(time.RFC3339Nano)},
        }
	manager := NewManager(nil)
	manager.SetProvider(provider)

	payload := CommandPayload{Request: CommandRequest{
		Operation: "create",
		Definition: &EntryDefinition{
			Name:     "test",
			Path:     "C:/tool.exe",
			Scope:    ScopeMachine,
			Source:   SourceRegistry,
                        Location: sampleLocation,
			Enabled:  true,
		},
	}}
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-create", Payload: marshalPayload(t, payload)})
	response := decodeResponse(t, result)
	if response.Operation != "create" {
		t.Fatalf("unexpected operation %q", response.Operation)
	}
	if provider.createReq == nil || provider.createReq.Definition.Path != "C:/tool.exe" {
		t.Fatalf("provider did not receive create request: %+v", provider.createReq)
	}
}

func TestManagerPropagatesErrors(t *testing.T) {
	provider := &fakeProvider{err: errors.New("denied")}
	manager := NewManager(nil)
	manager.SetProvider(provider)

	payload := CommandPayload{Request: CommandRequest{Operation: "remove", EntryID: "registry:machine:dGVzdA=="}}
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-remove", Payload: marshalPayload(t, payload)})
	if result.Success {
		t.Fatalf("expected failure result")
	}
	if result.Error != "denied" {
		t.Fatalf("unexpected error message %q", result.Error)
	}
}

func TestManagerRejectsInvalidToggleRequests(t *testing.T) {
	manager := NewManager(nil)
	payload := CommandPayload{Request: CommandRequest{Operation: "toggle", EntryID: "registry:machine:dGVzdA=="}}
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-toggle", Payload: marshalPayload(t, payload)})
	if result.Success {
		t.Fatalf("expected toggle without enabled flag to fail")
	}
}

func TestManagerRejectsUnsupportedOperations(t *testing.T) {
	manager := NewManager(nil)
	payload := CommandPayload{Request: CommandRequest{Operation: "unknown"}}
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-unknown", Payload: marshalPayload(t, payload)})
	if result.Success {
		t.Fatalf("expected unsupported operation to fail")
	}
}
