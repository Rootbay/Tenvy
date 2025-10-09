package pluginmanifest

import (
	"errors"
	"fmt"
	"strings"
)

type Manifest struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Entry        string            `json:"entry"`
	Author       string            `json:"author,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	License      string            `json:"license,omitempty"`
	Categories   []string          `json:"categories,omitempty"`
	Capabilities []Capability      `json:"capabilities,omitempty"`
	Requirements Requirements      `json:"requirements"`
	Distribution Distribution      `json:"distribution"`
	Package      PackageDescriptor `json:"package"`
}

type Capability struct {
	Name        string `json:"name"`
	Module      string `json:"module"`
	Description string `json:"description,omitempty"`
}

type Requirements struct {
	MinAgentVersion  string   `json:"minAgentVersion,omitempty"`
	MaxAgentVersion  string   `json:"maxAgentVersion,omitempty"`
	MinClientVersion string   `json:"minClientVersion,omitempty"`
	Platforms        []string `json:"platforms,omitempty"`
	Architectures    []string `json:"architectures,omitempty"`
	RequiredModules  []string `json:"requiredModules,omitempty"`
}

type Distribution struct {
	DefaultMode DeliveryMode `json:"defaultMode"`
	AutoUpdate  bool         `json:"autoUpdate"`
	Signature   Signature    `json:"signature"`
}

type PackageDescriptor struct {
	Artifact  string `json:"artifact"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
	Hash      string `json:"hash,omitempty"`
}

type Signature struct {
	Type      SignatureType `json:"type"`
	Hash      string        `json:"hash,omitempty"`
	PublicKey string        `json:"publicKey,omitempty"`
}

type (
	DeliveryMode  string
	SignatureType string
)

const (
	DeliveryManual    DeliveryMode  = "manual"
	DeliveryAutomatic DeliveryMode  = "automatic"
	SignatureNone     SignatureType = "none"
	SignatureSHA256   SignatureType = "sha256"
	SignatureEd25519  SignatureType = "ed25519"
)

func (m Manifest) Validate() error {
	var problems []error

	if strings.TrimSpace(m.ID) == "" {
		problems = append(problems, errors.New("missing id"))
	}
	if strings.TrimSpace(m.Name) == "" {
		problems = append(problems, errors.New("missing name"))
	}
	if strings.TrimSpace(m.Version) == "" {
		problems = append(problems, errors.New("missing version"))
	}
	if strings.TrimSpace(m.Entry) == "" {
		problems = append(problems, errors.New("missing entry"))
	}
	if strings.TrimSpace(m.Package.Artifact) == "" {
		problems = append(problems, errors.New("missing package artifact"))
	}

	if err := m.validateDistribution(); err != nil {
		problems = append(problems, err)
	}

	if len(m.Requirements.RequiredModules) > 0 {
		for index, module := range m.Requirements.RequiredModules {
			if strings.TrimSpace(module) == "" {
				problems = append(problems, fmt.Errorf("required module %d is empty", index))
			}
		}
	}

	return errors.Join(problems...)
}

func (m Manifest) validateDistribution() error {
	mode := strings.TrimSpace(string(m.Distribution.DefaultMode))
	if mode == "" {
		return errors.New("distribution default mode is required")
	}
	switch DeliveryMode(mode) {
	case DeliveryManual, DeliveryAutomatic:
	default:
		return fmt.Errorf("unsupported delivery mode: %s", mode)
	}

	switch m.Distribution.Signature.Type {
	case SignatureNone:
	case SignatureSHA256:
		if strings.TrimSpace(m.Distribution.Signature.Hash) == "" {
			return errors.New("sha256 signature requires hash")
		}
	case SignatureEd25519:
		if strings.TrimSpace(m.Distribution.Signature.PublicKey) == "" {
			return errors.New("ed25519 signature requires publicKey")
		}
		if strings.TrimSpace(m.Distribution.Signature.Hash) == "" {
			return errors.New("ed25519 signature requires hash")
		}
	default:
		return fmt.Errorf("unsupported signature type: %s", m.Distribution.Signature.Type)
	}

	return nil
}
