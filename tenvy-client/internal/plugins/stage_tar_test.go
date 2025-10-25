package plugins_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	plugins "github.com/rootbay/tenvy-client/internal/plugins"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
)

func TestStagePluginStagesTarGzArtifact(t *testing.T) {
	t.Parallel()

	artifactData := buildTarGzArchive(t, map[string][]byte{
		"awesome-plugin/plugin.bin": []byte("plugin payload"),
	})
	hash := sha256.Sum256(artifactData)
	hashHex := fmt.Sprintf("%x", hash[:])
	signatureValue := releaseSignatureFor(t, hashHex)

	manifestJSON := fmt.Sprintf(`{
                "id": "awesome-plugin",
                "name": "Awesome Plugin",
                "version": "1.2.3",
                "entry": "awesome-plugin/plugin.bin",
                "repositoryUrl": "https://github.com/rootbay/tenvy-client",
                "license": {"spdxId": "MIT"},
                "requirements": {},
                "distribution": {"defaultMode": "automatic", "autoUpdate": true, "signature": "ed25519", "signatureHash": "%[1]s", "signatureSigner": "%[2]s", "signatureValue": "%[3]s", "signatureTimestamp": "%[4]s"},
                "package": {"artifact": "awesome-plugin/awesome-plugin.tar.gz", "hash": "%[1]s"}
        }`, hashHex, releaseSigner, signatureValue, releaseSignedAtStamp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/artifact") {
			w.Header().Set("Content-Type", "application/octet-stream")
			if _, err := w.Write(artifactData); err != nil {
				t.Fatalf("write artifact: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, manifestJSON); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	opts := releaseVerifyOptions(t)
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), opts)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	digest := sha256.Sum256([]byte(manifestJSON))
	descriptor := manifest.ManifestDescriptor{
		PluginID:       "awesome-plugin",
		Version:        "1.2.3",
		ManifestDigest: fmt.Sprintf("%x", digest[:]),
		Distribution:   manifest.ManifestBriefing{DefaultMode: manifest.DeliveryAutomatic, AutoUpdate: true},
		ArtifactHash:   hashHex,
	}

	ctx := context.Background()
	result, err := plugins.StagePlugin(ctx, manager, server.Client(), server.URL, "agent-1", "", "stage-test", manifest.RuntimeFacts{}, descriptor)
	if err != nil {
		t.Fatalf("stage plugin: %v", err)
	}
	if !result.Updated {
		t.Fatalf("expected installation to be marked updated")
	}
	if strings.TrimSpace(result.EntryPath) == "" {
		t.Fatalf("expected entry path to be set")
	}

	payload, err := os.ReadFile(result.EntryPath)
	if err != nil {
		t.Fatalf("read entry payload: %v", err)
	}
	if string(payload) != "plugin payload" {
		t.Fatalf("unexpected entry payload %q", string(payload))
	}

	artifactPath := filepath.Join(manager.Root(), "awesome-plugin", "awesome-plugin", "awesome-plugin.tar.gz")
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("expected artifact persisted: %v", err)
	}

	if !strings.EqualFold(result.Manifest.ID, "awesome-plugin") {
		t.Fatalf("expected manifest id awesome-plugin, got %q", result.Manifest.ID)
	}
}

func TestStagePluginFailsOnMalformedTarGz(t *testing.T) {
	t.Parallel()

	artifactData := []byte("broken tar.gz data")
	hash := sha256.Sum256(artifactData)
	hashHex := fmt.Sprintf("%x", hash[:])
	signatureValue := releaseSignatureFor(t, hashHex)

	manifestJSON := fmt.Sprintf(`{
                "id": "broken-plugin",
                "name": "Broken Plugin",
                "version": "0.0.1",
                "entry": "broken-plugin/plugin.bin",
                "repositoryUrl": "https://github.com/rootbay/tenvy-client",
                "license": {"spdxId": "MIT"},
                "requirements": {},
                "distribution": {"defaultMode": "automatic", "autoUpdate": true, "signature": "ed25519", "signatureHash": "%[1]s", "signatureSigner": "%[2]s", "signatureValue": "%[3]s", "signatureTimestamp": "%[4]s"},
                "package": {"artifact": "broken-plugin/broken-plugin.tar.gz", "hash": "%[1]s"}
        }`, hashHex, releaseSigner, signatureValue, releaseSignedAtStamp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/artifact") {
			w.Header().Set("Content-Type", "application/octet-stream")
			if _, err := w.Write(artifactData); err != nil {
				t.Fatalf("write artifact: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, manifestJSON); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	opts := releaseVerifyOptions(t)
	manager, err := plugins.NewManager(root, log.New(io.Discard, "", 0), opts)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	digest := sha256.Sum256([]byte(manifestJSON))
	descriptor := manifest.ManifestDescriptor{
		PluginID:       "broken-plugin",
		Version:        "0.0.1",
		ManifestDigest: fmt.Sprintf("%x", digest[:]),
		Distribution:   manifest.ManifestBriefing{DefaultMode: manifest.DeliveryAutomatic, AutoUpdate: true},
		ArtifactHash:   hashHex,
	}

	_, err = plugins.StagePlugin(context.Background(), manager, server.Client(), server.URL, "agent-1", "", "stage-test", manifest.RuntimeFacts{}, descriptor)
	if err == nil {
		t.Fatal("expected staging to fail")
	}

	stageErr := &plugins.StageError{}
	if !strings.Contains(err.Error(), "artifact") {
		t.Fatalf("expected artifact failure, got %v", err)
	}
	if !errors.As(err, &stageErr) {
		t.Fatalf("expected stage error type, got %T", err)
	}
	if stageErr.Status() != manifest.InstallError {
		t.Fatalf("expected install error status, got %s", stageErr.Status())
	}
	if stageErr.Version() != "0.0.1" {
		t.Fatalf("expected version propagated, got %q", stageErr.Version())
	}
}
