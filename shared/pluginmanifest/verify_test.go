package pluginmanifest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"
)

func TestVerifySignatureSHA256AllowList(t *testing.T) {
	manifest := Manifest{
		Distribution: Distribution{
			Signature: SignatureSHA256,
		},
		Package: PackageDescriptor{Hash: "abcdef"},
	}

	result, err := VerifySignature(manifest, VerifyOptions{SHA256AllowList: []string{"ABCDEF"}})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.Trusted {
		t.Fatalf("expected trusted result")
	}
	if result.Hash != "abcdef" {
		t.Fatalf("unexpected hash: %s", result.Hash)
	}
}

func TestVerifySignatureSHA256Disallowed(t *testing.T) {
	manifest := Manifest{
		Distribution: Distribution{
			Signature: SignatureSHA256,
		},
		Package: PackageDescriptor{Hash: "abcdef"},
	}

	if _, err := VerifySignature(manifest, VerifyOptions{SHA256AllowList: []string{"123456"}}); err != ErrHashNotAllowed {
		t.Fatalf("expected ErrHashNotAllowed, got %v", err)
	}
}

func TestVerifySignatureHashMismatch(t *testing.T) {
	manifest := Manifest{
		Distribution: Distribution{
			Signature:     SignatureSHA256,
			SignatureHash: "123456",
		},
		Package: PackageDescriptor{Hash: "abcdef"},
	}

	if _, err := VerifySignature(manifest, VerifyOptions{}); err != ErrSignatureMismatch {
		t.Fatalf("expected ErrSignatureMismatch, got %v", err)
	}
}

func TestVerifySignatureMissingPackageHash(t *testing.T) {
	manifest := Manifest{
		Distribution: Distribution{
			Signature: SignatureSHA256,
		},
	}

	if _, err := VerifySignature(manifest, VerifyOptions{}); err == nil {
		t.Fatalf("expected error when package hash missing")
	}
}

func TestVerifySignatureEd25519(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	hash := "9e4cba26f4f913a52fcb11f16a34f1db493f9204f0545d01b7a086764d814176"
	sig := ed25519.Sign(priv, []byte(hash))

	manifest := Manifest{
		Distribution: Distribution{
			Signature:          SignatureEd25519,
			SignatureHash:      hash,
			SignatureSigner:    "key-1",
			SignatureValue:     hex.EncodeToString(sig),
			SignatureTimestamp: time.Now().UTC().Format(time.RFC3339),
		},
		Package: PackageDescriptor{Hash: hash},
	}

	result, err := VerifySignature(manifest, VerifyOptions{
		Ed25519PublicKeys: map[string]ed25519.PublicKey{"key-1": pub},
		MaxSignatureAge:   time.Hour,
		CurrentTime: func() time.Time {
			return time.Now().UTC().Add(30 * time.Minute)
		},
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.Trusted {
		t.Fatalf("expected trusted result")
	}
	expectedKey := hex.EncodeToString(pub)
	if result.PublicKey != expectedKey {
		t.Fatalf("unexpected public key: %s", result.PublicKey)
	}
}

func TestVerifySignatureEd25519CertificateValidation(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	hash := "0123456789abcdef"
	sig := ed25519.Sign(priv, []byte(hash))

	var validated bool

	manifest := Manifest{
		Distribution: Distribution{
			Signature:                 SignatureEd25519,
			SignatureHash:             hash,
			SignatureValue:            hex.EncodeToString(sig),
			SignatureSigner:           "key-2",
			SignatureCertificateChain: []string{"cert-a", "cert-b"},
		},
		Package: PackageDescriptor{Hash: hash},
	}

	_, err = VerifySignature(manifest, VerifyOptions{
		Ed25519PublicKeys: map[string]ed25519.PublicKey{"key-2": pub},
		CertificateValidator: func(chain []string) error {
			validated = true
			if len(chain) != 2 {
				t.Fatalf("unexpected chain length: %d", len(chain))
			}
			if chain[0] != "cert-a" || chain[1] != "cert-b" {
				t.Fatalf("unexpected chain contents: %v", chain)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !validated {
		t.Fatalf("expected certificate validator to run")
	}
}

func TestVerifySignatureEd25519UntrustedSigner(t *testing.T) {
	manifest := Manifest{
		Distribution: Distribution{
			Signature:       SignatureEd25519,
			SignatureHash:   "abcdef",
			SignatureSigner: "missing",
			SignatureValue:  hex.EncodeToString(make([]byte, ed25519.SignatureSize)),
		},
		Package: PackageDescriptor{Hash: "abcdef"},
	}

	if _, err := VerifySignature(manifest, VerifyOptions{}); err != ErrUntrustedSigner {
		t.Fatalf("expected ErrUntrustedSigner, got %v", err)
	}
}

func TestVerifySignatureExpired(t *testing.T) {
	manifest := Manifest{
		Distribution: Distribution{
			Signature:          SignatureSHA256,
			SignatureHash:      "abcdef",
			SignatureTimestamp: time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339),
		},
		Package: PackageDescriptor{Hash: "abcdef"},
	}

	_, err := VerifySignature(manifest, VerifyOptions{
		SHA256AllowList: []string{"abcdef"},
		MaxSignatureAge: 24 * time.Hour,
		CurrentTime: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != ErrSignatureExpired {
		t.Fatalf("expected ErrSignatureExpired, got %v", err)
	}
}
