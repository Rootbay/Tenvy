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
