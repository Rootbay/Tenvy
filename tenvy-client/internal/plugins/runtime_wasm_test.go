package plugins

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rootbay/tenvy-client/internal/plugins/testsupport"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

func buildWasmModule(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	modulePath := filepath.Join(tempDir, "plugin.wasm")
	if err := os.WriteFile(modulePath, testsupport.SandboxModule, 0o644); err != nil {
		t.Fatalf("write wasm module: %v", err)
	}
	return modulePath
}

func TestLaunchRuntimeWASM(t *testing.T) {
	modulePath := buildWasmModule(t)

	var buffer bytes.Buffer
	logger := log.New(&buffer, "", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	handle, err := LaunchRuntime(ctx, modulePath, RuntimeOptions{
		Kind:           RuntimeKindWASM,
		Name:           "test-wasm",
		Logger:         logger,
		HostInterfaces: []string{manifest.HostInterfaceCoreV1},
		HostAPIVersion: "1.0",
		Sandboxed:      true,
	})
	if err != nil {
		t.Fatalf("launch wasm runtime: %v", err)
	}

	t.Cleanup(func() {
		_ = handle.Shutdown(context.Background())
	})

	time.Sleep(50 * time.Millisecond)

	if err := handle.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown wasm runtime: %v", err)
	}

	if !strings.Contains(buffer.String(), "plugin runtime test-wasm log: hello") {
		t.Fatalf("expected log message in buffer, got %q", buffer.String())
	}

	if _, ok := handle.(*wasmRuntimeHandle); !ok {
		t.Fatalf("expected wasm runtime handle, got %T", handle)
	}
}

func TestLaunchRuntimeWASMStartFailure(t *testing.T) {
	modulePath := buildWasmModule(t)

	if err := os.WriteFile(modulePath, []byte{0x00, 0x61}, 0o644); err != nil {
		t.Fatalf("corrupt wasm module: %v", err)
	}

	_, err := LaunchRuntime(context.Background(), modulePath, RuntimeOptions{Kind: RuntimeKindWASM})
	if err == nil {
		t.Fatal("expected wasm launch to fail")
	}
}
