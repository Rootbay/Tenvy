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
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Version       string             `json:"version"`
	Description   string             `json:"description,omitempty"`
	Entry         string             `json:"entry"`
	Author        string             `json:"author,omitempty"`
	Homepage      string             `json:"homepage,omitempty"`
	RepositoryURL string             `json:"repositoryUrl,omitempty"`
	License       *LicenseInfo       `json:"license,omitempty"`
	Categories    []string           `json:"categories,omitempty"`
	Capabilities  []string           `json:"capabilities,omitempty"`
	Telemetry     []string           `json:"telemetry,omitempty"`
	Dependencies  []string           `json:"dependencies,omitempty"`
	Runtime       *RuntimeDescriptor `json:"runtime,omitempty"`
	Requirements  Requirements       `json:"requirements"`
	Distribution  Distribution       `json:"distribution"`
	Package       PackageDescriptor  `json:"package"`
}

type CapabilityMetadata struct {
	ID          string
	Module      string
	Name        string
	Description string
}

type TelemetryMetadata struct {
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

type RuntimeDescriptor struct {
	Type      RuntimeType          `json:"type"`
	Sandboxed bool                 `json:"sandboxed,omitempty"`
	Host      *RuntimeHostContract `json:"host,omitempty"`
}

type RuntimeHostContract struct {
	APIVersion string   `json:"apiVersion,omitempty"`
	Interfaces []string `json:"interfaces,omitempty"`
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
	RuntimeType          string
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

	RuntimeNative RuntimeType = "native"
	RuntimeWASM   RuntimeType = "wasm"

	HostInterfaceCoreV1 = "tenvy.core/1"
)

var (
	knownDeliveryModes  = []DeliveryMode{DeliveryManual, DeliveryAutomatic}
	knownSignatureTypes = []SignatureType{SignatureSHA256, SignatureEd25519}
	knownPlatforms      = []PluginPlatform{PlatformWindows, PlatformLinux, PlatformMacOS}
	knownArchitectures  = []PluginArchitecture{ArchitectureX8664, ArchitectureARM64}
	knownInstallStates  = []PluginInstallStatus{InstallBlocked, InstallDisabled, InstallError, InstallInstalled}
	knownApprovalStates = []PluginApprovalStatus{ApprovalPending, ApprovalApproved, ApprovalRejected}
	knownRuntimeTypes   = []RuntimeType{RuntimeNative, RuntimeWASM}
	semverPattern       = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-.]+)?(?:\+[0-9A-Za-z-.]+)?$`)
	registeredModules   = map[string]struct{}{
		"app-vnc":         {},
		"remote-desktop":  {},
		"webcam-control":  {},
		"audio-control":   {},
		"keylogger":       {},
		"clipboard":       {},
		"file-manager":    {},
		"task-manager":    {},
		"tcp-connections": {},
		"recovery":        {},
		"client-chat":     {},
		"system-info":     {},
		"notes":           {},
	}
	registeredCapabilities = map[string]CapabilityMetadata{
		"app-vnc.launch": {
			ID:          "app-vnc.launch",
			Module:      "app-vnc",
			Name:        "app-vnc.launch",
			Description: "Clone per-application profiles and start virtualized sessions.",
		},
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
		"webcam.enumerate": {
			ID:          "webcam.enumerate",
			Module:      "webcam-control",
			Name:        "webcam.enumerate",
			Description: "Enumerate connected webcam devices and capabilities.",
		},
		"webcam.stream": {
			ID:          "webcam.stream",
			Module:      "webcam-control",
			Name:        "webcam.stream",
			Description: "Initiate webcam streaming sessions when supported.",
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
		"keylogger.stream": {
			ID:          "keylogger.stream",
			Module:      "keylogger",
			Name:        "keylogger.stream",
			Description: "Stream keystroke telemetry to the controller in near real time.",
		},
		"keylogger.batch": {
			ID:          "keylogger.batch",
			Module:      "keylogger",
			Name:        "keylogger.batch",
			Description: "Batch keystrokes offline and upload on a schedule.",
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
		"file-manager.explore": {
			ID:          "file-manager.explore",
			Module:      "file-manager",
			Name:        "file-manager.explore",
			Description: "Enumerate directories and retrieve file contents from the host.",
		},
		"file-manager.modify": {
			ID:          "file-manager.modify",
			Module:      "file-manager",
			Name:        "file-manager.modify",
			Description: "Create, update, move, and delete files and directories on demand.",
		},
		"task-manager.list": {
			ID:          "task-manager.list",
			Module:      "task-manager",
			Name:        "task-manager.list",
			Description: "Collect real-time process snapshots with metadata.",
		},
		"task-manager.control": {
			ID:          "task-manager.control",
			Module:      "task-manager",
			Name:        "task-manager.control",
			Description: "Start and orchestrate process actions on demand.",
		},
		"tcp-connections.enumerate": {
			ID:          "tcp-connections.enumerate",
			Module:      "tcp-connections",
			Name:        "tcp-connections.enumerate",
			Description: "Collect real-time socket state with process attribution.",
		},
		"tcp-connections.control": {
			ID:          "tcp-connections.control",
			Module:      "tcp-connections",
			Name:        "tcp-connections.control",
			Description: "Stage enforcement actions for suspicious remote peers.",
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
	registeredTelemetry = map[string]TelemetryMetadata{
		"remote-desktop.metrics": {
			ID:          "remote-desktop.metrics",
			Module:      "remote-desktop",
			Name:        "Performance telemetry",
			Description: "Emit adaptive streaming metrics describing encoder performance and transport health.",
		},
		"audio.telemetry": {
			ID:          "audio.telemetry",
			Module:      "audio-control",
			Name:        "Audio telemetry",
			Description: "Report capture bridge levels, buffer health, and device availability to the controller.",
		},
		"system-info.telemetry": {
			ID:          "system-info.telemetry",
			Module:      "system-info",
			Name:        "System telemetry",
			Description: "Stream host performance counters, thermal states, and resource utilization snapshots.",
		},
	}
)

func LookupTelemetry(id string) (TelemetryMetadata, bool) {
	descriptor, ok := registeredTelemetry[strings.TrimSpace(id)]
	return descriptor, ok
}

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
	Dependencies   []string         `json:"dependencies,omitempty"`
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
	if runtimeErrs := m.validateRuntime(); len(runtimeErrs) > 0 {
		problems = append(problems, runtimeErrs...)
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

	dependencySeen := make(map[string]struct{})
	manifestID := strings.ToLower(strings.TrimSpace(m.ID))
	for index, dependency := range m.Dependencies {
		trimmed := strings.TrimSpace(dependency)
		if trimmed == "" {
			problems = append(problems, fmt.Errorf("dependency %d is empty", index))
			continue
		}
		lowered := strings.ToLower(trimmed)
		if lowered == manifestID && lowered != "" {
			problems = append(problems, fmt.Errorf("dependency %s cannot reference the plugin itself", trimmed))
			continue
		}
		if _, ok := dependencySeen[lowered]; ok {
			problems = append(problems, fmt.Errorf("dependency %s is duplicated", trimmed))
			continue
		}
		dependencySeen[lowered] = struct{}{}
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

	for index, telemetryID := range m.Telemetry {
		trimmed := strings.TrimSpace(telemetryID)
		if trimmed == "" {
			problems = append(problems, fmt.Errorf("telemetry %d is empty", index))
			continue
		}
		descriptor, ok := LookupTelemetry(trimmed)
		if !ok {
			problems = append(problems, fmt.Errorf("telemetry %s is not registered", trimmed))
			continue
		}
		if descriptor.Module == "" {
			continue
		}
		if _, ok := registeredModules[descriptor.Module]; !ok {
			problems = append(problems, fmt.Errorf("telemetry %s references unknown module %s", descriptor.ID, descriptor.Module))
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

func (m Manifest) validateRuntime() []error {
	descriptor := m.Runtime
	if descriptor == nil {
		return nil
	}

	var problems []error
	runtimeType := strings.TrimSpace(string(descriptor.Type))
	if runtimeType != "" {
		normalized := RuntimeType(strings.ToLower(runtimeType))
		if !containsRuntimeType(normalized) {
			problems = append(problems, fmt.Errorf("unsupported runtime type: %s", descriptor.Type))
		}
	}

	if descriptor.Host != nil {
		apiVersion := strings.TrimSpace(descriptor.Host.APIVersion)
		if apiVersion != "" && len(apiVersion) < 2 {
			problems = append(problems, fmt.Errorf("runtime host apiVersion is invalid: %s", descriptor.Host.APIVersion))
		}
		for index, iface := range descriptor.Host.Interfaces {
			if strings.TrimSpace(iface) == "" {
				problems = append(problems, fmt.Errorf("runtime host interface %d is empty", index))
			}
		}
	}

	return problems
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

func (m Manifest) RuntimeType() RuntimeType {
	if m.Runtime == nil {
		return RuntimeNative
	}
	raw := strings.ToLower(strings.TrimSpace(string(m.Runtime.Type)))
	switch raw {
	case string(RuntimeWASM):
		return RuntimeWASM
	case string(RuntimeNative):
		return RuntimeNative
	case "":
		return RuntimeNative
	default:
		return RuntimeNative
	}
}

func (m Manifest) RuntimeSandboxed() bool {
	if m.Runtime == nil {
		return false
	}
	return m.Runtime.Sandboxed
}

func (m Manifest) RuntimeHostInterfaces() []string {
	if m.Runtime == nil || m.Runtime.Host == nil {
		return nil
	}
	return sanitizeStringSlice(m.Runtime.Host.Interfaces)
}

func (m Manifest) RuntimeHostAPIVersion() string {
	if m.Runtime == nil || m.Runtime.Host == nil {
		return ""
	}
	return strings.TrimSpace(m.Runtime.Host.APIVersion)
}

func (m Manifest) DependenciesList() []string {
	return sanitizeStringSlice(m.Dependencies)
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

func containsRuntimeType(candidate RuntimeType) bool {
	return containsValue(candidate, knownRuntimeTypes)
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

func sanitizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		lowered := strings.ToLower(trimmed)
		if _, ok := seen[lowered]; ok {
			continue
		}
		seen[lowered] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	sort.Strings(normalized)
	return normalized
}

func init() {
	sort.Slice(knownDeliveryModes, func(i, j int) bool { return knownDeliveryModes[i] < knownDeliveryModes[j] })
	sort.Slice(knownSignatureTypes, func(i, j int) bool { return knownSignatureTypes[i] < knownSignatureTypes[j] })
	sort.Slice(knownPlatforms, func(i, j int) bool { return knownPlatforms[i] < knownPlatforms[j] })
	sort.Slice(knownArchitectures, func(i, j int) bool { return knownArchitectures[i] < knownArchitectures[j] })
	sort.Slice(knownInstallStates, func(i, j int) bool { return knownInstallStates[i] < knownInstallStates[j] })
	sort.Slice(knownApprovalStates, func(i, j int) bool { return knownApprovalStates[i] < knownApprovalStates[j] })
	sort.Slice(knownRuntimeTypes, func(i, j int) bool { return knownRuntimeTypes[i] < knownRuntimeTypes[j] })
}
