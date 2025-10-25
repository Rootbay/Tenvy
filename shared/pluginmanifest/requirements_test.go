package pluginmanifest

import "testing"

func TestCheckRuntimeCompatibilityRequiresSupportedRuntime(t *testing.T) {
	manifest := buildTestManifest()
	manifest.Runtime = &RuntimeDescriptor{Type: RuntimeWASM, Host: &RuntimeHostContract{Interfaces: []string{HostInterfaceCoreV1}}}

	facts := RuntimeFacts{
		Platform:          "windows",
		Architecture:      "x86_64",
		AgentVersion:      "1.0.0",
		EnabledModules:    []string{"clipboard"},
		SupportedRuntimes: []RuntimeType{RuntimeNative},
		HostInterfaces:    []string{HostInterfaceCoreV1},
		HostAPIVersion:    "1.0",
	}

	if err := CheckRuntimeCompatibility(manifest, facts); err == nil {
		t.Fatal("expected runtime compatibility check to fail when wasm is unsupported")
	}

	facts.SupportedRuntimes = []RuntimeType{RuntimeNative, RuntimeWASM}
	if err := CheckRuntimeCompatibility(manifest, facts); err != nil {
		t.Fatalf("unexpected runtime compatibility error: %v", err)
	}
}

func TestCheckRuntimeCompatibilityRequiresHostInterface(t *testing.T) {
	manifest := buildTestManifest()
	manifest.Runtime = &RuntimeDescriptor{Type: RuntimeWASM, Host: &RuntimeHostContract{Interfaces: []string{"tenvy.experimental/1"}}}

	facts := RuntimeFacts{
		Platform:          "windows",
		Architecture:      "x86_64",
		AgentVersion:      "1.0.0",
		EnabledModules:    []string{"clipboard"},
		SupportedRuntimes: []RuntimeType{RuntimeNative, RuntimeWASM},
		HostInterfaces:    []string{HostInterfaceCoreV1},
		HostAPIVersion:    "1.0",
	}

	if err := CheckRuntimeCompatibility(manifest, facts); err == nil {
		t.Fatal("expected compatibility check to fail for missing host interface")
	}

	manifest.Runtime.Host.Interfaces = []string{HostInterfaceCoreV1}
	if err := CheckRuntimeCompatibility(manifest, facts); err != nil {
		t.Fatalf("unexpected host interface compatibility error: %v", err)
	}
}
