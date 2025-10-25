package pluginmanifest

import (
	"strings"
	"testing"
)

func buildTestManifest() Manifest {
	hash := strings.Repeat("a", 64)
	return Manifest{
		ID:            "test-plugin",
		Name:          "Test Plugin",
		Version:       "1.2.3",
		Entry:         "plugin.exe",
		RepositoryURL: "https://example.com/test-plugin",
		Distribution: Distribution{
			DefaultMode:   DeliveryAutomatic,
			AutoUpdate:    true,
			Signature:     SignatureSHA256,
			SignatureHash: hash,
		},
		Package: PackageDescriptor{
			Artifact: "plugin.zip",
			Hash:     hash,
		},
		Requirements: Requirements{
			Platforms:     []PluginPlatform{PlatformWindows},
			Architectures: []PluginArchitecture{ArchitectureX8664},
		},
	}
}

func TestManifestValidateAllowsArtifactFileName(t *testing.T) {
	manifest := buildTestManifest()

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected validation success, got %v", err)
	}
}

func TestManifestValidateRejectsPathQualifiedArtifact(t *testing.T) {
	manifest := buildTestManifest()
	manifest.Package.Artifact = "nested/plugin.zip"

	err := manifest.Validate()
	if err == nil {
		t.Fatal("expected validation to fail for path-qualified artifact")
	}
	if !strings.Contains(err.Error(), "package artifact must be a file name") {
		t.Fatalf("expected artifact file name error, got %v", err)
	}
}

func TestManifestValidateTelemetry(t *testing.T) {
	manifest := buildTestManifest()
	manifest.Telemetry = []string{"remote-desktop.metrics"}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected telemetry validation success, got %v", err)
	}

	manifest.Telemetry = []string{"unknown.telemetry"}
	err := manifest.Validate()
	if err == nil {
		t.Fatal("expected telemetry validation to fail for unknown descriptor")
	}
	if !strings.Contains(err.Error(), "telemetry unknown.telemetry is not registered") {
		t.Fatalf("expected unknown telemetry error, got %v", err)
	}
}

func TestManifestValidateRuntime(t *testing.T) {
	manifest := buildTestManifest()
	manifest.Runtime = &RuntimeDescriptor{
		Type:      RuntimeWASM,
		Sandboxed: true,
		Host: &RuntimeHostContract{
			Interfaces: []string{HostInterfaceCoreV1},
			APIVersion: "1.0",
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected runtime validation success, got %v", err)
	}

	manifest.Runtime.Host.Interfaces = []string{"", HostInterfaceCoreV1}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected runtime validation to fail for empty interface")
	}
}

func TestManifestValidateDependencies(t *testing.T) {
	manifest := buildTestManifest()
	manifest.Dependencies = []string{"helper-plugin"}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected dependency validation success, got %v", err)
	}

	manifest.Dependencies = []string{"helper-plugin", "HELPER-plugin"}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected duplicate dependency validation failure")
	}

	manifest.Dependencies = []string{"test-plugin"}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected self dependency validation failure")
	}
}

func TestManifestValidateRecognizesRegisteredModulesAndCapabilities(t *testing.T) {
	for moduleID := range registeredModules {
		manifest := buildTestManifest()
		manifest.Requirements.RequiredModules = []string{moduleID}

		if err := manifest.Validate(); err != nil {
			t.Fatalf("expected module %s to be registered, got %v", moduleID, err)
		}
	}

	for capabilityID := range registeredCapabilities {
		manifest := buildTestManifest()
		manifest.Capabilities = []string{capabilityID}

		if err := manifest.Validate(); err != nil {
			t.Fatalf("expected capability %s to be registered, got %v", capabilityID, err)
		}
	}

	for telemetryID := range registeredTelemetry {
		manifest := buildTestManifest()
		manifest.Telemetry = []string{telemetryID}

		if err := manifest.Validate(); err != nil {
			t.Fatalf("expected telemetry %s to be registered, got %v", telemetryID, err)
		}
	}
}
