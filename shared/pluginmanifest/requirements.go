package pluginmanifest

import (
	"fmt"
	"sort"
	"strings"
)

type RuntimeFacts struct {
	Platform       string
	Architecture   string
	AgentVersion   string
	EnabledModules []string
}

func CheckRuntimeCompatibility(m Manifest, facts RuntimeFacts) error {
	label := strings.TrimSpace(m.Name)
	if label == "" {
		label = strings.TrimSpace(m.ID)
	}
	if label == "" {
		label = "plugin"
	}

	requirements := m.Requirements

	if err := evaluatePlatformRequirement(label, requirements.Platforms, facts.Platform); err != nil {
		return err
	}
	if err := evaluateArchitectureRequirement(label, requirements.Architectures, facts.Architecture); err != nil {
		return err
	}
	if err := evaluateVersionRequirements(label, requirements.MinAgentVersion, requirements.MaxAgentVersion, facts.AgentVersion); err != nil {
		return err
	}
	if err := evaluateModuleRequirements(label, requirements.RequiredModules, facts.EnabledModules); err != nil {
		return err
	}
	return nil
}

func evaluatePlatformRequirement(label string, platforms []PluginPlatform, platform string) error {
	if len(platforms) == 0 {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(platform))
	if normalized == "" {
		return fmt.Errorf("%s requires platform %s but runtime platform is unknown", label, joinStringSlice(platformSliceToStrings(platforms)))
	}
	for _, candidate := range platforms {
		if normalized == strings.ToLower(strings.TrimSpace(string(candidate))) {
			return nil
		}
	}
	return fmt.Errorf("%s requires platform %s but runtime platform is %s", label, joinStringSlice(platformSliceToStrings(platforms)), normalized)
}

func evaluateArchitectureRequirement(label string, architectures []PluginArchitecture, architecture string) error {
	if len(architectures) == 0 {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(architecture))
	if normalized == "" {
		return fmt.Errorf("%s requires architecture %s but runtime architecture is unknown", label, joinStringSlice(architectureSliceToStrings(architectures)))
	}
	for _, candidate := range architectures {
		if normalized == strings.ToLower(strings.TrimSpace(string(candidate))) {
			return nil
		}
	}
	return fmt.Errorf("%s requires architecture %s but runtime architecture is %s", label, joinStringSlice(architectureSliceToStrings(architectures)), normalized)
}

func evaluateVersionRequirements(label, minVersion, maxVersion, runtimeVersion string) error {
	runtimeVersion = strings.TrimSpace(runtimeVersion)
	minVersion = strings.TrimSpace(minVersion)
	maxVersion = strings.TrimSpace(maxVersion)

	if minVersion != "" {
		if runtimeVersion == "" {
			return fmt.Errorf("%s requires agent version >= %s but runtime version is unknown", label, minVersion)
		}
		cmp, err := compareSemver(runtimeVersion, minVersion)
		if err != nil {
			return fmt.Errorf("%s has invalid agent version %q: %w", label, runtimeVersion, err)
		}
		if cmp < 0 {
			return fmt.Errorf("%s requires agent version >= %s but runtime version is %s", label, minVersion, runtimeVersion)
		}
	}

	if maxVersion != "" {
		if runtimeVersion == "" {
			return fmt.Errorf("%s requires agent version <= %s but runtime version is unknown", label, maxVersion)
		}
		cmp, err := compareSemver(runtimeVersion, maxVersion)
		if err != nil {
			return fmt.Errorf("%s has invalid agent version %q: %w", label, runtimeVersion, err)
		}
		if cmp > 0 {
			return fmt.Errorf("%s requires agent version <= %s but runtime version is %s", label, maxVersion, runtimeVersion)
		}
	}

	return nil
}

func evaluateModuleRequirements(label string, required, enabled []string) error {
	if len(required) == 0 {
		return nil
	}
	normalized := make(map[string]struct{}, len(enabled))
	for _, module := range enabled {
		trimmed := strings.ToLower(strings.TrimSpace(module))
		if trimmed == "" {
			continue
		}
		normalized[trimmed] = struct{}{}
	}

	var missing []string
	for _, module := range required {
		trimmed := strings.ToLower(strings.TrimSpace(module))
		if trimmed == "" {
			continue
		}
		if _, ok := normalized[trimmed]; !ok {
			missing = append(missing, strings.TrimSpace(module))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	if len(enabled) == 0 {
		return fmt.Errorf("%s requires agent module(s) %s but no modules are active", label, joinStringSlice(missing))
	}
	return fmt.Errorf("%s requires agent module(s) %s but only %s are active", label, joinStringSlice(missing), joinStringSlice(enabled))
}

func joinStringSlice(values []string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	if len(filtered) == 0 {
		return "(none)"
	}
	return strings.Join(filtered, ", ")
}

func platformSliceToStrings(values []PluginPlatform) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}

func architectureSliceToStrings(values []PluginArchitecture) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}

func compareSemver(left, right string) (int, error) {
	lhs, err := parseSemver(left)
	if err != nil {
		return 0, err
	}
	rhs, err := parseSemver(right)
	if err != nil {
		return 0, err
	}
	if lhs.major != rhs.major {
		if lhs.major < rhs.major {
			return -1, nil
		}
		return 1, nil
	}
	if lhs.minor != rhs.minor {
		if lhs.minor < rhs.minor {
			return -1, nil
		}
		return 1, nil
	}
	if lhs.patch != rhs.patch {
		if lhs.patch < rhs.patch {
			return -1, nil
		}
		return 1, nil
	}
	if lhs.prerelease == rhs.prerelease {
		return 0, nil
	}
	if lhs.prerelease == "" {
		return 1, nil
	}
	if rhs.prerelease == "" {
		return -1, nil
	}
	if lhs.prerelease < rhs.prerelease {
		return -1, nil
	}
	if lhs.prerelease > rhs.prerelease {
		return 1, nil
	}
	return 0, nil
}

type semverParts struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func parseSemver(value string) (semverParts, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return semverParts{}, fmt.Errorf("version is empty")
	}
	if !semverPattern.MatchString(trimmed) {
		return semverParts{}, fmt.Errorf("version %q is not a semantic version", value)
	}
	withoutBuild := trimmed
	if idx := strings.Index(withoutBuild, "+"); idx >= 0 {
		withoutBuild = withoutBuild[:idx]
	}
	prerelease := ""
	core := withoutBuild
	if idx := strings.Index(core, "-"); idx >= 0 {
		prerelease = core[idx+1:]
		core = core[:idx]
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semverParts{}, fmt.Errorf("version %q is not a semantic version", value)
	}
	major, err := parseNumericComponent(parts[0])
	if err != nil {
		return semverParts{}, err
	}
	minor, err := parseNumericComponent(parts[1])
	if err != nil {
		return semverParts{}, err
	}
	patch, err := parseNumericComponent(parts[2])
	if err != nil {
		return semverParts{}, err
	}
	return semverParts{major: major, minor: minor, patch: patch, prerelease: prerelease}, nil
}

func parseNumericComponent(value string) (int, error) {
	var n int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid numeric component: %s", value)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
