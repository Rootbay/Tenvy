package pluginmanifest

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrUnsignedPlugin       = errors.New("plugin manifest is unsigned")
	ErrHashNotAllowed       = errors.New("plugin manifest hash not in allow list")
	ErrSignatureMismatch    = errors.New("plugin signature does not match manifest hash")
	ErrUntrustedSigner      = errors.New("plugin manifest signer is not trusted")
	ErrInvalidSignature     = errors.New("plugin signature is invalid")
	ErrSignatureExpired     = errors.New("plugin signature is expired")
	ErrSignatureNotYetValid = errors.New("plugin signature timestamp is in the future")
)

type CertificateChainValidator func(chain []string) error

type Ed25519PublicKeyResolver func(keyID string) (ed25519.PublicKey, bool, error)

type VerifyOptions struct {
	SHA256AllowList         []string
	Ed25519PublicKeys       map[string]ed25519.PublicKey
	ResolveEd25519PublicKey Ed25519PublicKeyResolver
	CertificateValidator    CertificateChainValidator
	MaxSignatureAge         time.Duration
	CurrentTime             func() time.Time
}

type VerificationResult struct {
	Trusted          bool
	SignatureType    SignatureType
	Hash             string
	PublicKey        string
	Signer           string
	SignedAt         *time.Time
	CertificateChain []string
}

func VerifySignature(manifest Manifest, opts VerifyOptions) (*VerificationResult, error) {
	sigType := strings.TrimSpace(string(manifest.Distribution.Signature))
	if sigType == "" {
		return nil, ErrUnsignedPlugin
	}

	if !containsSignatureType(SignatureType(sigType)) {
		return nil, fmt.Errorf("unsupported signature type: %s", sigType)
	}

	nowFn := opts.CurrentTime
	if nowFn == nil {
		nowFn = time.Now
	}

	var signedAtPtr *time.Time
	if strings.TrimSpace(manifest.Distribution.SignatureTimestamp) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(manifest.Distribution.SignatureTimestamp))
		if err != nil {
			return nil, fmt.Errorf("plugin manifest signedAt is invalid: %w", err)
		}
		signedAtPtr = &parsed
		if opts.MaxSignatureAge > 0 {
			age := nowFn().Sub(parsed)
			if age < 0 {
				return nil, ErrSignatureNotYetValid
			}
			if age > opts.MaxSignatureAge {
				return nil, ErrSignatureExpired
			}
		}
	} else if opts.MaxSignatureAge > 0 {
		return nil, ErrSignatureExpired
	}

	normalizedManifestHash := normalizeHexString(manifest.Package.Hash)
	if normalizedManifestHash == "" {
		return nil, fmt.Errorf("plugin manifest is missing package hash")
	}

	normalizedSignatureHash := normalizeHexString(manifest.Distribution.SignatureHash)
	if normalizedSignatureHash != "" && normalizedManifestHash != normalizedSignatureHash {
		return nil, ErrSignatureMismatch
	}

	hashValue := normalizedManifestHash

	switch SignatureType(sigType) {
	case SignatureSHA256:
		if len(opts.SHA256AllowList) > 0 {
			if !hashAllowed(hashValue, opts.SHA256AllowList) {
				return nil, ErrHashNotAllowed
			}
		}
		return &VerificationResult{
			Trusted:          true,
			SignatureType:    SignatureSHA256,
			Hash:             hashValue,
			Signer:           strings.TrimSpace(manifest.Distribution.SignatureSigner),
			SignedAt:         signedAtPtr,
			CertificateChain: append([]string(nil), manifest.Distribution.SignatureCertificateChain...),
		}, nil
	case SignatureEd25519:
		signerID := strings.TrimSpace(manifest.Distribution.SignatureSigner)
		if signerID == "" {
			return nil, ErrUntrustedSigner
		}
		signatureValue := strings.TrimSpace(manifest.Distribution.SignatureValue)
		if signatureValue == "" {
			return nil, ErrInvalidSignature
		}

		publicKey, err := resolveEd25519Key(signerID, opts)
		if err != nil {
			return nil, err
		}
		if len(publicKey) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("plugin manifest public key has invalid length: %d", len(publicKey))
		}

		signatureBytes, err := hex.DecodeString(signatureValue)
		if err != nil {
			return nil, fmt.Errorf("plugin signature is not valid hex: %w", err)
		}
		if len(signatureBytes) != ed25519.SignatureSize {
			return nil, fmt.Errorf("plugin signature has invalid length: %d", len(signatureBytes))
		}

		message := []byte(hashValue)
		if !ed25519.Verify(publicKey, message, signatureBytes) {
			return nil, ErrInvalidSignature
		}

		if opts.CertificateValidator != nil && len(manifest.Distribution.SignatureCertificateChain) > 0 {
			if err := opts.CertificateValidator(append([]string(nil), manifest.Distribution.SignatureCertificateChain...)); err != nil {
				return nil, fmt.Errorf("certificate chain validation failed: %w", err)
			}
		}
		return &VerificationResult{
			Trusted:          true,
			SignatureType:    SignatureEd25519,
			Hash:             hashValue,
			PublicKey:        hex.EncodeToString(publicKey),
			Signer:           signerID,
			SignedAt:         signedAtPtr,
			CertificateChain: append([]string(nil), manifest.Distribution.SignatureCertificateChain...),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported signature type: %s", sigType)
	}
}

func hashAllowed(hash string, allowList []string) bool {
	if hash == "" {
		return false
	}
	for _, candidate := range allowList {
		if hash == normalizeHexString(candidate) {
			return true
		}
	}
	return false
}

func resolveEd25519Key(signerID string, opts VerifyOptions) (ed25519.PublicKey, error) {
	trimmedKeyID := strings.TrimSpace(signerID)
	if trimmedKeyID == "" {
		return nil, ErrUntrustedSigner
	}

	if opts.ResolveEd25519PublicKey != nil {
		key, ok, err := opts.ResolveEd25519PublicKey(trimmedKeyID)
		if err != nil {
			return nil, fmt.Errorf("resolve ed25519 public key: %w", err)
		}
		if ok {
			return append(ed25519.PublicKey(nil), key...), nil
		}
	}

	if opts.Ed25519PublicKeys != nil {
		if key, ok := opts.Ed25519PublicKeys[trimmedKeyID]; ok {
			return append(ed25519.PublicKey(nil), key...), nil
		}
	}

	return nil, ErrUntrustedSigner
}

func normalizeHexString(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
