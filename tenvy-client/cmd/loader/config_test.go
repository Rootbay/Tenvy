package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"testing"
)

func TestParseEmbeddedRuntimeConfigModules(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	payload := runtimeConfigPayload{
		Modules: []string{" remote-desktop ", "remote-desktop", "system-info"},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	result := parseEmbeddedRuntimeConfig(logger, encoded)
	if result.Modules == nil {
		t.Fatal("expected modules to be parsed, got nil")
	}
	expected := []string{"remote-desktop", "system-info"}
	if len(result.Modules) != len(expected) {
		t.Fatalf("unexpected module count: got %v want %v", result.Modules, expected)
	}
	for index := range expected {
		if result.Modules[index] != expected[index] {
			t.Fatalf("module mismatch at %d: got %q want %q", index, result.Modules[index], expected[index])
		}
	}
}

func TestParseEmbeddedRuntimeConfigModulesOptional(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	emptyPayload := base64.StdEncoding.EncodeToString([]byte(`{"modules":[]}`))
	emptyResult := parseEmbeddedRuntimeConfig(logger, emptyPayload)
	if emptyResult.Modules == nil {
		t.Fatal("expected empty slice when modules provided as empty array")
	}
	if len(emptyResult.Modules) != 0 {
		t.Fatalf("expected empty module list, got %v", emptyResult.Modules)
	}

	absentPayload := base64.StdEncoding.EncodeToString([]byte(`{}`))
	absentResult := parseEmbeddedRuntimeConfig(logger, absentPayload)
	if absentResult.Modules != nil {
		t.Fatalf("expected nil modules when not provided, got %v", absentResult.Modules)
	}
}
