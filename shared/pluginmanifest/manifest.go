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
	RepositoryURL string            `json:"repositoryUrl"`
	License       LicenseInfo       `json:"license"`
	Categories    []string          `json:"categories,omitempty"`
	Capabilities  []Capability      `json:"capabilities,omitempty"`
	Requirements  Requirements      `json:"requirements"`
	Distribution  Distribution      `json:"distribution"`
	Package       PackageDescriptor `json:"package"`
}

type Capability struct {
	Name        string `json:"name"`
	Module      string `json:"module"`
	Description string `json:"description,omitempty"`
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
	Signature string        `json:"signature,omitempty"`
	SignedAt  string        `json:"signedAt,omitempty"`
	Signer    string        `json:"signer,omitempty"`
	Chain     []string      `json:"certificateChain,omitempty"`
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

	InstallPending    PluginInstallStatus = "pending"
	InstallInstalling PluginInstallStatus = "installing"
	InstallInstalled  PluginInstallStatus = "installed"
	InstallFailed     PluginInstallStatus = "failed"
	InstallBlocked    PluginInstallStatus = "blocked"

	ApprovalPending  PluginApprovalStatus = "pending"
	ApprovalApproved PluginApprovalStatus = "approved"
	ApprovalRejected PluginApprovalStatus = "rejected"
)

var (
	knownDeliveryModes  = []DeliveryMode{DeliveryManual, DeliveryAutomatic}
	knownSignatureTypes = []SignatureType{SignatureSHA256, SignatureEd25519}
	knownPlatforms      = []PluginPlatform{PlatformWindows, PlatformLinux, PlatformMacOS}
	knownArchitectures  = []PluginArchitecture{ArchitectureX8664, ArchitectureARM64}
	knownInstallStates  = []PluginInstallStatus{InstallPending, InstallInstalling, InstallInstalled, InstallFailed, InstallBlocked}
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
)

type InstallationTelemetry struct {
	PluginID       string              `json:"pluginId"`
	Version        string              `json:"version"`
	Status         PluginInstallStatus `json:"status"`
	Hash           string              `json:"hash,omitempty"`
	LastDeployedAt *string             `json:"lastDeployedAt,omitempty"`
	LastCheckedAt  *string             `json:"lastCheckedAt,omitempty"`
	Error          string              `json:"error,omitempty"`
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
	if err := validateGitHubRepository(m.RepositoryURL); err != nil {
		problems = append(problems, err)
	}
	if err := m.validateLicense(); err != nil {
		problems = append(problems, err)
	}
	if strings.TrimSpace(m.Package.Artifact) == "" {
		problems = append(problems, errors.New("missing package artifact"))
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

	for index, capability := range m.Capabilities {
		if strings.TrimSpace(capability.Name) == "" {
			problems = append(problems, fmt.Errorf("capability %d is missing name", index))
		}
		if strings.TrimSpace(capability.Module) == "" {
			problems = append(problems, fmt.Errorf("capability %s is missing module reference", capability.Name))
			continue
		}
		if _, ok := registeredModules[capability.Module]; !ok {
			problems = append(problems, fmt.Errorf("capability %s references unknown module %s", capability.Name, capability.Module))
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

	sig := m.Distribution.Signature
	if !containsSignatureType(sig.Type) {
		return fmt.Errorf("unsupported signature type: %s", sig.Type)
	}

	switch sig.Type {
	case SignatureSHA256:
		if strings.TrimSpace(sig.Hash) == "" {
			return errors.New("sha256 signature requires hash")
		}
	case SignatureEd25519:
		if strings.TrimSpace(sig.PublicKey) == "" {
			return errors.New("ed25519 signature requires publicKey")
		}
		if strings.TrimSpace(sig.Hash) == "" {
			return errors.New("ed25519 signature requires hash")
		}
	}

	if strings.TrimSpace(m.Package.Hash) == "" {
		return errors.New("signed packages must include a hash")
	}
	if strings.TrimSpace(sig.Signature) == "" {
		return errors.New("signed manifests must provide signature value")
	}

	return nil
}

func (m Manifest) validateLicense() error {
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

func validateGitHubRepository(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return errors.New("repositoryUrl is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("repositoryUrl invalid: %v", err)
	}
	if parsed.Scheme != "https" {
		return errors.New("repositoryUrl must use https")
	}
	if !strings.EqualFold(parsed.Host, "github.com") {
		return errors.New("repositoryUrl must reference github.com")
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 {
		return errors.New("repositoryUrl must include owner and repository")
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
