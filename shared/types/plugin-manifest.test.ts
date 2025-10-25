import { describe, expect, it } from "vitest";
import {
  type PluginManifest,
  validatePluginManifest,
} from "./plugin-manifest.js";

const baseManifest: PluginManifest = {
  id: "test-plugin",
  name: "Test Plugin",
  version: "1.2.3",
  entry: "plugin.exe",
  repositoryUrl: "https://example.com/test-plugin",
  requirements: {
    platforms: ["windows"],
    architectures: ["x86_64"],
    requiredModules: [],
  },
  distribution: {
    defaultMode: "automatic",
    autoUpdate: true,
    signature: "sha256",
    signatureHash: "a".repeat(64),
  },
  package: {
    artifact: "plugin.zip",
    hash: "a".repeat(64),
  },
};

const cloneManifest = (): PluginManifest =>
  JSON.parse(JSON.stringify(baseManifest)) as PluginManifest;

describe("validatePluginManifest", () => {
  it("accepts artifact file names without path separators", () => {
    const manifest = cloneManifest();

    const problems = validatePluginManifest(manifest);

    expect(problems).toHaveLength(0);
  });

  it("rejects artifact paths containing directory separators", () => {
    const manifest = cloneManifest();
    manifest.package.artifact = "nested/plugin.zip";

    const problems = validatePluginManifest(manifest);

    expect(problems).toContain("package artifact must be a file name");
  });

  it("validates telemetry descriptors", () => {
    const manifest = cloneManifest();
    manifest.telemetry = ["remote-desktop.metrics"];

    let problems = validatePluginManifest(manifest);
    expect(problems).not.toContain(
      "telemetry remote-desktop.metrics is not registered",
    );

    manifest.telemetry = ["unknown.telemetry"];
    problems = validatePluginManifest(manifest);

    expect(problems).toContain("telemetry unknown.telemetry is not registered");
  });

  it("accepts wasm runtime descriptors", () => {
    const manifest = cloneManifest();
    manifest.runtime = {
      type: "wasm",
      sandboxed: true,
      host: { interfaces: ["tenvy.core/1"], apiVersion: "1.0" },
    };

    const problems = validatePluginManifest(manifest);

    expect(problems).toHaveLength(0);
  });

  it("rejects unsupported runtime metadata", () => {
    const manifest = cloneManifest();
    manifest.runtime = {
      type: "invalid-type" as never,
      host: { interfaces: ["", "tenvy.core/1"], apiVersion: "" },
    };

    const problems = validatePluginManifest(manifest);

    expect(problems).toContain("unsupported runtime type: invalid-type");
    expect(problems).toContain("runtime host interface 0 is empty");
    expect(problems).toContain("runtime host apiVersion cannot be empty");
  });

  it("validates plugin dependency lists", () => {
    const manifest = cloneManifest();
    manifest.dependencies = ["helper-plugin"];

    let problems = validatePluginManifest(manifest);
    expect(problems).not.toContain("dependency helper-plugin is duplicated");

    manifest.dependencies = ["helper-plugin", "HELPER-plugin"];
    problems = validatePluginManifest(manifest);
    expect(problems).toContain("dependency HELPER-plugin is duplicated");

    manifest.dependencies = ["test-plugin"];
    problems = validatePluginManifest(manifest);
    expect(problems).toContain(
      "dependency test-plugin cannot reference the plugin itself",
    );
  });
});
