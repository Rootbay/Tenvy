package registry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeProvider struct {
	listReq         *ListRequest
	listResp        RegistryListResult
	createKeyReq    *CreateKeyRequest
	createKeyResp   RegistryMutationResult
	createValueReq  *CreateValueRequest
	createValueResp RegistryMutationResult
	updateKeyReq    *UpdateKeyRequest
	updateKeyResp   RegistryMutationResult
	updateValueReq  *UpdateValueRequest
	updateValueResp RegistryMutationResult
	deleteKeyReq    *DeleteKeyRequest
	deleteKeyResp   RegistryMutationResult
	deleteValueReq  *DeleteValueRequest
	deleteValueResp RegistryMutationResult
	err             error
	caps            ProviderCapabilities
}

func (f *fakeProvider) List(ctx context.Context, req ListRequest) (RegistryListResult, error) {
	f.listReq = &req
	return f.listResp, f.err
}

func (f *fakeProvider) CreateKey(ctx context.Context, req CreateKeyRequest) (RegistryMutationResult, error) {
	f.createKeyReq = &req
	return f.createKeyResp, f.err
}

func (f *fakeProvider) CreateValue(ctx context.Context, req CreateValueRequest) (RegistryMutationResult, error) {
	f.createValueReq = &req
	return f.createValueResp, f.err
}

func (f *fakeProvider) UpdateKey(ctx context.Context, req UpdateKeyRequest) (RegistryMutationResult, error) {
	f.updateKeyReq = &req
	return f.updateKeyResp, f.err
}

func (f *fakeProvider) UpdateValue(ctx context.Context, req UpdateValueRequest) (RegistryMutationResult, error) {
	f.updateValueReq = &req
	return f.updateValueResp, f.err
}

func (f *fakeProvider) DeleteKey(ctx context.Context, req DeleteKeyRequest) (RegistryMutationResult, error) {
	f.deleteKeyReq = &req
	return f.deleteKeyResp, f.err
}

func (f *fakeProvider) DeleteValue(ctx context.Context, req DeleteValueRequest) (RegistryMutationResult, error) {
	f.deleteValueReq = &req
	return f.deleteValueResp, f.err
}

func (f *fakeProvider) Capabilities() ProviderCapabilities {
	return f.caps
}

func marshalPayload(t *testing.T, payload RegistryCommandPayload) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

func parseResponse(t *testing.T, result CommandResult) RegistryCommandResponse {
	t.Helper()
	if !result.Success {
		t.Fatalf("expected successful command result, got error %q", result.Error)
	}
	var decoded RegistryCommandResponse
	if err := json.Unmarshal([]byte(result.Output), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return decoded
}

func TestManagerDispatchesListRequests(t *testing.T) {
	provider := &fakeProvider{
		listResp: RegistryListResult{
			Snapshot: RegistrySnapshot{
				"HKEY_CURRENT_USER": RegistryHive{
					"Software": {
						Hive:         "HKEY_CURRENT_USER",
						Name:         "Software",
						Path:         "Software",
						ParentPath:   nil,
						Values:       nil,
						SubKeys:      nil,
						LastModified: time.Now().UTC().Format(time.RFC3339Nano),
						Owner:        "TEN\\Analyst",
					},
				},
			},
			GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	manager := NewManager(nil)
	manager.SetProvider(provider)

	payload := RegistryCommandPayload{
		Request: RegistryCommandRequest{Operation: "list", Hive: "HKEY_CURRENT_USER"},
	}
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-1", Payload: marshalPayload(t, payload)})
	response := parseResponse(t, result)
	if response.Operation != "list" || response.Status != "ok" {
		t.Fatalf("unexpected response metadata: %+v", response)
	}
	if provider.listReq == nil || provider.listReq.Hive != "HKEY_CURRENT_USER" {
		t.Fatalf("provider did not receive list request: %+v", provider.listReq)
	}
}

func TestManagerHandlesCreateKeyRequests(t *testing.T) {
	provider := &fakeProvider{
		createKeyResp: RegistryMutationResult{
			Hive:      RegistryHive{"Root": {Path: "Root"}},
			KeyPath:   "Root\\New",
			MutatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	manager := NewManager(nil)
	manager.SetProvider(provider)

	payload := RegistryCommandPayload{
		Request: RegistryCommandRequest{
			Operation:  "create",
			Target:     "key",
			Hive:       "HKEY_LOCAL_MACHINE",
			ParentPath: "Root",
			Name:       "New",
		},
	}

	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-2", Payload: marshalPayload(t, payload)})
	response := parseResponse(t, result)
	if response.Operation != "create" {
		t.Fatalf("expected create operation, got %s", response.Operation)
	}
	if provider.createKeyReq == nil || provider.createKeyReq.Name != "New" {
		t.Fatalf("provider did not receive key creation request: %+v", provider.createKeyReq)
	}
}

func TestManagerPropagatesProviderErrors(t *testing.T) {
	provider := &fakeProvider{err: errors.New("denied")}
	manager := NewManager(nil)
	manager.SetProvider(provider)

	payload := RegistryCommandPayload{
		Request: RegistryCommandRequest{
			Operation: "delete",
			Target:    "key",
			Hive:      "HKEY_USERS",
			Path:      "Sample",
		},
	}
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-err", Payload: marshalPayload(t, payload)})
	if result.Success {
		t.Fatalf("expected failure result")
	}
	if result.Error != "denied" {
		t.Fatalf("unexpected error message: %s", result.Error)
	}
}

func TestManagerRejectsUnsupportedOperations(t *testing.T) {
	manager := NewManager(nil)
	payload := RegistryCommandPayload{
		Request: RegistryCommandRequest{Operation: "unknown"},
	}
	result := manager.HandleCommand(context.Background(), Command{ID: "cmd-3", Payload: marshalPayload(t, payload)})
	if result.Success {
		t.Fatalf("expected unsupported operation to fail")
	}
}
