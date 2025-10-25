package pluginmanifest

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

type Manifest struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Description   string            `json:"description,omitempty"`
	Entry         string            `json:"entry"`
	Author        string            `json:"author,omitempty"`
	Homepage      string            `json:"homepage,omitempty"`
	RepositoryURL string            `json:"repositoryUrl,omitempty"`
	License       *LicenseInfo      `json:"license,omitempty"`
	Categories    []string          `json:"categories,omitempty"`
	Capabilities  []string          `json:"capabilities,omitempty"`
	Requirements  Requirements      `json:"requirements"`
	Distribution  Distribution      `json:"distribution"`
	Package       PackageDescriptor `json:"package"`
}

type CapabilityMetadata struct {
	ID          string
	Module      string
	Name        string
	Description string
}

type Requirements struct {
	MinAgentVersion  string               `json:"minAgentVersion,omitempty"`
	MaxAgentVersion  string               `json:"maxAgentVersion,omitempty"`
	MinClientVersion string               `json:"minClientVersion,omitempty"`
	Platforms        []PluginPlatform     `json:"platforms,omitempty"`
	Architectures    []PluginArchitecture `json:"architectures,omitempty"`
	RequiredModules  []string             `json:"requiredModules,omitempty"`
}

type Distribution struct {
	DefaultMode               DeliveryMode  `json:"defaultMode"`
	AutoUpdate                bool          `json:"autoUpdate"`
	Signature                 SignatureType `json:"signature"`
	SignatureHash             string        `json:"signatureHash,omitempty"`
	SignatureValue            string        `json:"signatureValue,omitempty"`
	SignatureTimestamp        string        `json:"signatureTimestamp,omitempty"`
	SignatureSigner           string        `json:"signatureSigner,omitempty"`
	SignatureCertificateChain []string      `json:"signatureCertificateChain,omitempty"`
}

type PackageDescriptor struct {
	Artifact  string `json:"artifact"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
	Hash      string `json:"hash,omitempty"`
}

type LicenseInfo struct {
	SPDXID string `json:"spdxId"`
	Name   string `json:"name,omitempty"`
	URL    string `json:"url,omitempty"`
}

type (
	DeliveryMode         string
	SignatureType        string
	PluginPlatform       string
	PluginArchitecture   string
	PluginInstallStatus  string
	PluginApprovalStatus string
)

const (
	DeliveryManual    DeliveryMode = "manual"
	DeliveryAutomatic DeliveryMode = "automatic"

	SignatureSHA256  SignatureType = "sha256"
	SignatureEd25519 SignatureType = "ed25519"

	PlatformWindows PluginPlatform = "windows"
	PlatformLinux   PluginPlatform = "linux"
	PlatformMacOS   PluginPlatform = "macos"

	ArchitectureX8664 PluginArchitecture = "x86_64"
	ArchitectureARM64 PluginArchitecture = "arm64"

	InstallInstalled PluginInstallStatus = "installed"
	InstallBlocked   PluginInstallStatus = "blocked"
	InstallError     PluginInstallStatus = "error"
	InstallDisabled  PluginInstallStatus = "disabled"

	ApprovalPending  PluginApprovalStatus = "pending"
	ApprovalApproved PluginApprovalStatus = "approved"
	ApprovalRejected PluginApprovalStatus = "rejected"
)

var (
	knownDeliveryModes  = []DeliveryMode{DeliveryManual, DeliveryAutomatic}
	knownSignatureTypes = []SignatureType{SignatureSHA256, SignatureEd25519}
	knownPlatforms      = []PluginPlatform{PlatformWindows, PlatformLinux, PlatformMacOS}
	knownArchitectures  = []PluginArchitecture{ArchitectureX8664, ArchitectureARM64}
	knownInstallStates  = []PluginInstallStatus{InstallBlocked, InstallDisabled, InstallError, InstallInstalled}
	knownApprovalStates = []PluginApprovalStatus{ApprovalPending, ApprovalApproved, ApprovalRejected}
	semverPattern       = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-.]+)?(?:\+[0-9A-Za-z-.]+)?$`)
	registeredModules   = map[string]struct{}{
		"remote-desktop": {},
		"audio-control":  {},
		"clipboard":      {},
		"recovery":       {},
		"client-chat":    {},
		"system-info":    {},
		"notes":          {},
	}
	registeredCapabilities = map[string]CapabilityMetadata{
		"remote-desktop.stream": {
			ID:          "remote-desktop.stream",
			Module:      "remote-desktop",
			Name:        "Desktop streaming",
			Description: "Stream high-fidelity desktop frames to the controller UI.",
		},
		"remote-desktop.input": {
			ID:          "remote-desktop.input",
			Module:      "remote-desktop",
			Name:        "Input relay",
			Description: "Relay keyboard and pointer events back to the remote host.",
		},
		"remote-desktop.transport.quic": {
			ID:          "remote-desktop.transport.quic",
			Module:      "remote-desktop",
			Name:        "QUIC transport",
			Description: "Provide QUIC transport negotiation for resilient input streams.",
		},
		"remote-desktop.codec.hevc": {
			ID:          "remote-desktop.codec.hevc",
			Module:      "remote-desktop",
			Name:        "HEVC encoding",
			Description: "Enable hardware-accelerated HEVC streaming when supported.",
		},
		"remote-desktop.metrics": {
			ID:          "remote-desktop.metrics",
			Module:      "remote-desktop",
			Name:        "Performance telemetry",
			Description: "Collect frame quality and adaptive bitrate metrics for dashboards.",
		},
		"audio.capture": {
			ID:          "audio.capture",
			Module:      "audio-control",
			Name:        "Audio capture",
			Description: "Capture remote system audio for monitoring and recording.",
		},
		"audio.inject": {
			ID:          "audio.inject",
			Module:      "audio-control",
			Name:        "Audio injection",
			Description: "Inject operator-provided audio streams into the remote session.",
		},
		"clipboard.capture": {
			ID:          "clipboard.capture",
			Module:      "clipboard",
			Name:        "Clipboard capture",
			Description: "Capture clipboard changes emitted by the remote workstation.",
		},
		"clipboard.push": {
			ID:          "clipboard.push",
			Module:      "clipboard",
			Name:        "Clipboard push",
			Description: "Push operator clipboard payloads to the remote host.",
		},
		"recovery.queue": {
			ID:          "recovery.queue",
			Module:      "recovery",
			Name:        "Recovery queue",
			Description: "Queue recovery jobs for background execution and monitoring.",
		},
		"recovery.collect": {
			ID:          "recovery.collect",
			Module:      "recovery",
			Name:        "Artifact collection",
			Description: "Collect artifacts staged by upstream modules for exfiltration.",
		},
		"vault.export": {
			ID:          "vault.export",
			Module:      "recovery",
			Name:        "Vault export collection",
			Description: "Stage and exfiltrate vault exports via the recovery pipeline.",
		},
		"client-chat.persistent": {
			ID:          "client-chat.persistent",
			Module:      "client-chat",
			Name:        "Persistent window",
			Description: "Keep the chat interface open continuously and respawn it if terminated.",
		},
		"client-chat.alias": {
			ID:          "client-chat.alias",
			Module:      "client-chat",
			Name:        "Alias control",
			Description: "Allow the controller to update operator and client aliases in real time.",
		},
		"system-info.snapshot": {
			ID:          "system-info.snapshot",
			Module:      "system-info",
			Name:        "System snapshot",
			Description: "Produce structured operating system and hardware inventories.",
		},
		"system-info.telemetry": {
			ID:          "system-info.telemetry",
			Module:      "system-info",
			Name:        "System telemetry",
			Description: "Surface live telemetry metrics used by scheduling and recovery modules.",
		},
		"vault.enumerate": {
			ID:          "vault.enumerate",
			Module:      "system-info",
			Name:        "Vault enumeration",
			Description: "Enumerate installed password managers and browser credential stores.",
		},
		"notes.sync": {
			ID:          "notes.sync",
			Module:      "notes",
			Name:        "Notes sync",
			Description: "Synchronize local incident notes to the operator vault with delta compression.",
		},
	}
)

type InstallationTelemetry struct {
	PluginID  string              `json:"pluginId"`
	Version   string              `json:"version"`
	Status    PluginInstallStatus `json:"status"`
	Hash      string              `json:"hash,omitempty"`
	Timestamp *int64              `json:"timestamp,omitempty"`
	Error     string              `json:"error,omitempty"`
}

type SyncPayload struct {
	Installations []InstallationTelemetry `json:"installations"`
	Manifests     *ManifestState          `json:"manifests,omitempty"`
}

type ManifestDescriptor struct {
	PluginID       string           `json:"pluginId"`
	Version        string           `json:"version"`
	ManifestDigest string           `json:"manifestDigest"`
	ArtifactHash   string           `json:"artifactHash,omitempty"`
	ArtifactSize   int64            `json:"artifactSizeBytes,omitempty"`
	ApprovedAt     string           `json:"approvedAt,omitempty"`
	ManualPushAt   string           `json:"manualPushAt,omitempty"`
	Distribution   ManifestBriefing `json:"distribution"`
}

type ManifestBriefing struct {
	DefaultMode DeliveryMode `json:"defaultMode"`
	AutoUpdate  bool         `json:"autoUpdate"`
}

type ManifestList struct {
	Version   string               `json:"version"`
	Manifests []ManifestDescriptor `json:"manifests"`
}

type ManifestState struct {
	Version string            `json:"version,omitempty"`
	Digests map[string]string `json:"digests,omitempty"`
}

type ManifestDelta struct {
	Version string               `json:"version"`
	Updated []ManifestDescriptor `json:"updated"`
	Removed []string             `json:"removed"`
}

func (m Manifest) Validate() error {
	var problems []error

	if strings.TrimSpace(m.ID) == "" {
		problems = append(problems, errors.New("missing id"))
	}
	if strings.TrimSpace(m.Name) == "" {
		problems = append(problems, errors.New("missing name"))
	}
	if version := strings.TrimSpace(m.Version); version == "" {
		problems = append(problems, errors.New("missing version"))
	} else if !semverPattern.MatchString(version) {
		problems = append(problems, fmt.Errorf("invalid semantic version: %s", m.Version))
	}
	if strings.TrimSpace(m.Entry) == "" {
		problems = append(problems, errors.New("missing entry"))
	}
	if err := validateRepositoryURL(m.RepositoryURL); err != nil {
		problems = append(problems, err)
	}
	if err := m.validateLicense(); err != nil {
		problems = append(problems, err)
	}
        artifact := strings.TrimSpace(m.Package.Artifact)
        if artifact == "" {
                problems = append(problems, errors.New("missing package artifact"))
        } else if strings.ContainsAny(artifact, "/\\") {
                problems = append(problems, errors.New("package artifact must be a file name"))
        }

	if err := m.validateDistribution(); err != nil {
		problems = append(problems, err)
	}

	for index, module := range m.Requirements.RequiredModules {
		if strings.TrimSpace(module) == "" {
			problems = append(problems, fmt.Errorf("required module %d is empty", index))
			continue
		}
		if _, ok := registeredModules[module]; !ok {
			problems = append(problems, fmt.Errorf("required module %s is not registered", module))
		}
	}

	for index, capabilityID := range m.Capabilities {
		trimmed := strings.TrimSpace(capabilityID)
		if trimmed == "" {
			problems = append(problems, fmt.Errorf("capability %d is empty", index))
			continue
		}
		descriptor, ok := LookupCapability(trimmed)
		if !ok {
			problems = append(problems, fmt.Errorf("capability %s is not registered", trimmed))
			continue
		}
		if descriptor.Module == "" {
			continue
		}
		if _, ok := registeredModules[descriptor.Module]; !ok {
			problems = append(problems, fmt.Errorf("capability %s references unknown module %s", descriptor.ID, descriptor.Module))
		}
	}

	if err := validateSemverConstraint("minAgentVersion", m.Requirements.MinAgentVersion); err != nil {
		problems = append(problems, err)
	}
	if err := validateSemverConstraint("maxAgentVersion", m.Requirements.MaxAgentVersion); err != nil {
		problems = append(problems, err)
	}
	if err := validateSemverConstraint("minClientVersion", m.Requirements.MinClientVersion); err != nil {
		problems = append(problems, err)
	}

	for _, platform := range m.Requirements.Platforms {
		if !containsPlatform(platform) {
			problems = append(problems, fmt.Errorf("unsupported platform: %s", platform))
		}
	}

	for _, arch := range m.Requirements.Architectures {
		if !containsArchitecture(arch) {
			problems = append(problems, fmt.Errorf("unsupported architecture: %s", arch))
		}
	}

	return errors.Join(problems...)
}

func (m Manifest) validateDistribution() error {
	mode := strings.TrimSpace(string(m.Distribution.DefaultMode))
	if mode == "" {
		return errors.New("distribution default mode is required")
	}
	if !containsDeliveryMode(DeliveryMode(mode)) {
		return fmt.Errorf("unsupported delivery mode: %s", mode)
	}

	sigType := strings.TrimSpace(string(m.Distribution.Signature))
	if sigType == "" {
		return errors.New("distribution signature is required")
	}
	if !containsSignatureType(SignatureType(sigType)) {
		return fmt.Errorf("unsupported signature type: %s", sigType)
	}

	packageHash := strings.TrimSpace(m.Package.Hash)
	if packageHash == "" {
		return errors.New("signed packages must include a hash")
	}

	if sigHash := strings.TrimSpace(m.Distribution.SignatureHash); sigHash != "" && !strings.EqualFold(sigHash, packageHash) {
		return errors.New("signature hash does not match package hash")
	}

	if SignatureType(sigType) == SignatureEd25519 {
		if strings.TrimSpace(m.Distribution.SignatureSigner) == "" {
			return errors.New("ed25519 signatures require a signer id")
		}
		if strings.TrimSpace(m.Distribution.SignatureValue) == "" {
			return errors.New("ed25519 signatures require a signature value")
		}
	}

	return nil
}

func (m Manifest) validateLicense() error {
	if m.License == nil {
		return nil
	}
	if strings.TrimSpace(m.License.SPDXID) == "" {
		return errors.New("license requires spdxId")
	}
	if trimmed := strings.TrimSpace(m.License.URL); trimmed != "" {
		parsed, err := url.Parse(trimmed)
		if err != nil || !parsed.IsAbs() {
			return fmt.Errorf("license url invalid: %s", m.License.URL)
		}
	}
	return nil
}

func LookupCapability(id string) (CapabilityMetadata, bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return CapabilityMetadata{}, false
	}
	if descriptor, ok := registeredCapabilities[trimmed]; ok {
		return descriptor, true
	}
	lowered := strings.ToLower(trimmed)
	if descriptor, ok := registeredCapabilities[lowered]; ok {
		return descriptor, true
	}
	return CapabilityMetadata{}, false
}

func validateRepositoryURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("repositoryUrl invalid: %v", err)
	}
	if !parsed.IsAbs() {
		return errors.New("repositoryUrl must be an absolute URL")
	}
	if parsed.Scheme != "https" {
		return errors.New("repositoryUrl must use https")
	}
	return nil
}

func containsDeliveryMode(candidate DeliveryMode) bool {
	return containsValue(candidate, knownDeliveryModes)
}

func containsSignatureType(candidate SignatureType) bool {
	return containsValue(candidate, knownSignatureTypes)
}

func containsPlatform(candidate PluginPlatform) bool {
	return containsValue(candidate, knownPlatforms)
}

func containsArchitecture(candidate PluginArchitecture) bool {
	return containsValue(candidate, knownArchitectures)
}

func containsInstallStatus(candidate PluginInstallStatus) bool {
	return containsValue(candidate, knownInstallStates)
}

func containsApprovalStatus(candidate PluginApprovalStatus) bool {
	return containsValue(candidate, knownApprovalStates)
}

func containsValue[T comparable](candidate T, values []T) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func validateSemverConstraint(field string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if !semverPattern.MatchString(trimmed) {
		return fmt.Errorf("invalid %s: %s", field, value)
	}
	return nil
}

func init() {
	sort.Slice(knownDeliveryModes, func(i, j int) bool { return knownDeliveryModes[i] < knownDeliveryModes[j] })
	sort.Slice(knownSignatureTypes, func(i, j int) bool { return knownSignatureTypes[i] < knownSignatureTypes[j] })
	sort.Slice(knownPlatforms, func(i, j int) bool { return knownPlatforms[i] < knownPlatforms[j] })
	sort.Slice(knownArchitectures, func(i, j int) bool { return knownArchitectures[i] < knownArchitectures[j] })
	sort.Slice(knownInstallStates, func(i, j int) bool { return knownInstallStates[i] < knownInstallStates[j] })
	sort.Slice(knownApprovalStates, func(i, j int) bool { return knownApprovalStates[i] < knownApprovalStates[j] })
}
