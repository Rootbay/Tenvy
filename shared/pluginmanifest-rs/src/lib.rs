use semver::Version;
use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::fmt;
use thiserror::Error;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PluginDeliveryMode {
    Manual,
    Automatic,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PluginSignatureType {
    Sha256,
    Ed25519,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PluginSignatureStatus {
    Trusted,
    Untrusted,
    Unsigned,
    Invalid,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PluginPlatform {
    Windows,
    Linux,
    Macos,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PluginArchitecture {
    X86_64,
    Arm64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PluginRuntimeType {
    Native,
    Wasm,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum PluginRuntimeHostInterface {
    #[serde(alias = "tenvy.core/1")]
    TenvyCoreV1,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PluginInstallStatus {
    Installed,
    Blocked,
    Error,
    Disabled,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum PluginApprovalStatus {
    Pending,
    Approved,
    Rejected,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(default)]
pub struct PluginRequirements {
    pub min_agent_version: Option<String>,
    pub max_agent_version: Option<String>,
    pub min_client_version: Option<String>,
    pub platforms: Vec<PluginPlatform>,
    pub architectures: Vec<PluginArchitecture>,
    pub required_modules: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginDistribution {
    pub default_mode: PluginDeliveryMode,
    #[serde(default)]
    pub auto_update: bool,
    pub signature: PluginSignatureType,
    #[serde(default)]
    pub signature_hash: Option<String>,
    #[serde(default)]
    pub signature_value: Option<String>,
    #[serde(default)]
    pub signature_signer: Option<String>,
    #[serde(default)]
    pub signature_timestamp: Option<String>,
    #[serde(default)]
    pub signature_certificate_chain: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginLicenseInfo {
    #[serde(default)]
    pub spdx_id: Option<String>,
    #[serde(default)]
    pub name: Option<String>,
    #[serde(default)]
    pub url: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginPackageDescriptor {
    pub artifact: String,
    #[serde(default)]
    pub size_bytes: Option<i64>,
    #[serde(default)]
    pub hash: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginRuntimeHostContract {
    #[serde(default)]
    pub api_version: Option<String>,
    #[serde(default)]
    pub interfaces: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginRuntimeDescriptor {
    #[serde(default)]
    pub r#type: Option<PluginRuntimeType>,
    #[serde(default)]
    pub sandboxed: Option<bool>,
    #[serde(default)]
    pub host: Option<PluginRuntimeHostContract>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginManifest {
    pub id: String,
    pub name: String,
    pub version: String,
    #[serde(default)]
    pub description: Option<String>,
    pub entry: String,
    #[serde(default)]
    pub author: Option<String>,
    #[serde(default)]
    pub homepage: Option<String>,
    #[serde(default)]
    pub repository_url: Option<String>,
    #[serde(default)]
    pub license: Option<PluginLicenseInfo>,
    #[serde(default)]
    pub categories: Vec<String>,
    #[serde(default)]
    pub capabilities: Vec<String>,
    #[serde(default)]
    pub telemetry: Vec<String>,
    #[serde(default)]
    pub dependencies: Vec<String>,
    #[serde(default)]
    pub runtime: Option<PluginRuntimeDescriptor>,
    pub requirements: PluginRequirements,
    pub distribution: PluginDistribution,
    #[serde(rename = "package")]
    pub package_descriptor: PluginPackageDescriptor,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(default)]
pub struct AgentPluginManifestState {
    pub version: Option<String>,
    pub digests: std::collections::BTreeMap<String, String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginManifestDescriptorDistribution {
    pub default_mode: PluginDeliveryMode,
    pub auto_update: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginManifestDescriptor {
    #[serde(rename = "pluginId")]
    pub plugin_id: String,
    pub version: String,
    #[serde(rename = "manifestDigest")]
    pub manifest_digest: String,
    #[serde(default, rename = "artifactHash")]
    pub artifact_hash: Option<String>,
    #[serde(default, rename = "artifactSizeBytes")]
    pub artifact_size_bytes: Option<i64>,
    #[serde(default, rename = "approvedAt")]
    pub approved_at: Option<String>,
    #[serde(default, rename = "manualPushAt")]
    pub manual_push_at: Option<String>,
    #[serde(default)]
    pub dependencies: Vec<String>,
    pub distribution: PluginManifestDescriptorDistribution,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginInstallationTelemetry {
    #[serde(rename = "pluginId")]
    pub plugin_id: String,
    pub version: String,
    pub status: PluginInstallStatus,
    #[serde(default)]
    pub hash: Option<String>,
    #[serde(default)]
    pub timestamp: Option<i64>,
    #[serde(default)]
    pub error: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(default)]
pub struct PluginSyncPayload {
    pub installations: Vec<PluginInstallationTelemetry>,
    #[serde(default)]
    pub manifests: Option<AgentPluginManifestState>,
}

#[derive(Debug, Clone, PartialEq, Eq, Error)]
pub enum ManifestValidationError {
    #[error("field `{field}` is missing or blank")]
    MissingValue { field: &'static str },
    #[error("field `{field}` contains an invalid semantic version: {value}")]
    InvalidSemver { field: &'static str, value: String },
    #[error("module `{module}` is not registered")]
    UnknownModule { module: String },
    #[error("capability `{capability}` is not registered")]
    UnknownCapability { capability: String },
    #[error("telemetry `{telemetry}` is not registered")]
    UnknownTelemetry { telemetry: String },
    #[error("field `{field}` has an invalid value: {message}")]
    InvalidValue {
        field: &'static str,
        message: String,
    },
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ValidationErrors {
    errors: Vec<ManifestValidationError>,
}

impl ValidationErrors {
    pub fn new(errors: Vec<ManifestValidationError>) -> Self {
        Self { errors }
    }

    pub fn errors(&self) -> &[ManifestValidationError] {
        &self.errors
    }

    pub fn into_errors(self) -> Vec<ManifestValidationError> {
        self.errors
    }

    pub fn is_empty(&self) -> bool {
        self.errors.is_empty()
    }
}

impl fmt::Display for ValidationErrors {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "plugin manifest validation failed")?;
        if !self.errors.is_empty() {
            write!(f, ":")?;
            for (index, error) in self.errors.iter().enumerate() {
                if index == 0 {
                    write!(f, " {error}")?;
                } else {
                    write!(f, "; {error}")?;
                }
            }
        }
        Ok(())
    }
}

impl std::error::Error for ValidationErrors {}

#[derive(Debug, Clone, Default)]
pub struct ValidationContext {
    module_ids: HashSet<String>,
    capability_ids: HashSet<String>,
    telemetry_ids: HashSet<String>,
}

impl ValidationContext {
    pub fn new<M, C, T>(modules: M, capabilities: C, telemetry: T) -> Self
    where
        M: IntoIterator,
        M::Item: Into<String>,
        C: IntoIterator,
        C::Item: Into<String>,
        T: IntoIterator,
        T::Item: Into<String>,
    {
        Self {
            module_ids: modules.into_iter().map(Into::into).collect(),
            capability_ids: capabilities.into_iter().map(Into::into).collect(),
            telemetry_ids: telemetry.into_iter().map(Into::into).collect(),
        }
    }

    pub fn contains_module(&self, value: &str) -> bool {
        self.module_ids.contains(value)
    }

    pub fn contains_capability(&self, value: &str) -> bool {
        self.capability_ids.contains(value)
    }

    pub fn contains_telemetry(&self, value: &str) -> bool {
        self.telemetry_ids.contains(value)
    }
}

fn validate_semver(field: &'static str, value: &str, errors: &mut Vec<ManifestValidationError>) {
    if Version::parse(value).is_err() {
        errors.push(ManifestValidationError::InvalidSemver {
            field,
            value: value.to_string(),
        });
    }
}

fn validate_required_string(
    field: &'static str,
    value: &str,
    errors: &mut Vec<ManifestValidationError>,
) {
    if value.trim().is_empty() {
        errors.push(ManifestValidationError::MissingValue { field });
    }
}

fn validate_hex(
    field: &'static str,
    value: &str,
    length: Option<usize>,
    errors: &mut Vec<ManifestValidationError>,
) {
    let trimmed = value.trim();
    let expected_len = length.unwrap_or_else(|| trimmed.len());
    let is_hex = trimmed.chars().all(|c| c.is_ascii_hexdigit());
    if !is_hex || trimmed.len() != expected_len {
        errors.push(ManifestValidationError::InvalidValue {
            field,
            message: format!("expected {expected_len}-character hexadecimal string"),
        });
    }
}

fn validate_modules(
    field: &'static str,
    values: &[String],
    ctx: &ValidationContext,
    errors: &mut Vec<ManifestValidationError>,
) {
    for module in values {
        let trimmed = module.trim();
        if trimmed.is_empty() {
            errors.push(ManifestValidationError::MissingValue { field });
            continue;
        }
        if !ctx.contains_module(trimmed) {
            errors.push(ManifestValidationError::UnknownModule {
                module: trimmed.to_string(),
            });
        }
    }
}

fn validate_capabilities(
    field: &'static str,
    values: &[String],
    ctx: &ValidationContext,
    errors: &mut Vec<ManifestValidationError>,
) {
    for capability in values {
        let trimmed = capability.trim();
        if trimmed.is_empty() {
            errors.push(ManifestValidationError::MissingValue { field });
            continue;
        }
        if !ctx.contains_capability(trimmed) {
            errors.push(ManifestValidationError::UnknownCapability {
                capability: trimmed.to_string(),
            });
        }
    }
}

fn validate_telemetry(
    field: &'static str,
    values: &[String],
    ctx: &ValidationContext,
    errors: &mut Vec<ManifestValidationError>,
) {
    for telemetry in values {
        let trimmed = telemetry.trim();
        if trimmed.is_empty() {
            errors.push(ManifestValidationError::MissingValue { field });
            continue;
        }
        if !ctx.contains_telemetry(trimmed) {
            errors.push(ManifestValidationError::UnknownTelemetry {
                telemetry: trimmed.to_string(),
            });
        }
    }
}

fn validate_package(package: &PluginPackageDescriptor, errors: &mut Vec<ManifestValidationError>) {
    validate_required_string("package.artifact", &package.artifact, errors);
    if let Some(size) = package.size_bytes {
        if size <= 0 {
            errors.push(ManifestValidationError::InvalidValue {
                field: "package.sizeBytes",
                message: "size must be greater than zero".into(),
            });
        }
    }
    if let Some(hash) = &package.hash {
        validate_hex("package.hash", hash, Some(64), errors);
    }
}

fn validate_distribution(
    distribution: &PluginDistribution,
    errors: &mut Vec<ManifestValidationError>,
) {
    match distribution.signature {
        PluginSignatureType::Sha256 => match distribution.signature_hash.as_deref() {
            Some(hash) => validate_hex("distribution.signatureHash", hash, Some(64), errors),
            None => errors.push(ManifestValidationError::MissingValue {
                field: "distribution.signatureHash",
            }),
        },
        PluginSignatureType::Ed25519 => {
            if distribution
                .signature_value
                .as_deref()
                .map(str::trim)
                .unwrap_or_default()
                .is_empty()
            {
                errors.push(ManifestValidationError::MissingValue {
                    field: "distribution.signatureValue",
                });
            }
        }
    }
}

fn validate_requirements(
    requirements: &PluginRequirements,
    errors: &mut Vec<ManifestValidationError>,
) {
    if let Some(version) = &requirements.min_agent_version {
        validate_semver("requirements.minAgentVersion", version, errors);
    }
    if let Some(version) = &requirements.max_agent_version {
        validate_semver("requirements.maxAgentVersion", version, errors);
    }
    if let Some(version) = &requirements.min_client_version {
        validate_semver("requirements.minClientVersion", version, errors);
    }
}

pub fn validate_manifest(
    manifest: &PluginManifest,
    ctx: &ValidationContext,
) -> Result<(), ValidationErrors> {
    let mut errors = Vec::new();

    validate_required_string("id", &manifest.id, &mut errors);
    validate_required_string("name", &manifest.name, &mut errors);
    validate_required_string("version", &manifest.version, &mut errors);
    validate_required_string("entry", &manifest.entry, &mut errors);

    if !manifest.version.trim().is_empty() {
        validate_semver("version", &manifest.version, &mut errors);
    }

    validate_requirements(&manifest.requirements, &mut errors);
    validate_distribution(&manifest.distribution, &mut errors);
    validate_package(&manifest.package_descriptor, &mut errors);

    validate_modules(
        "requirements.requiredModules",
        &manifest.requirements.required_modules,
        ctx,
        &mut errors,
    );
    validate_capabilities("capabilities", &manifest.capabilities, ctx, &mut errors);
    validate_telemetry("telemetry", &manifest.telemetry, ctx, &mut errors);

    if errors.is_empty() {
        Ok(())
    } else {
        Err(ValidationErrors::new(errors))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn context() -> ValidationContext {
        ValidationContext::new(
            ["core.system-info", "core.remote-desktop"],
            ["capability.system-info.view"],
            ["telemetry.system-info"],
        )
    }

    fn base_manifest() -> PluginManifest {
        PluginManifest {
            id: "plugin.remote-desktop".into(),
            name: "Remote desktop".into(),
            version: "1.2.3".into(),
            description: Some("Enables remote desktop control".into()),
            entry: "remote-desktop.dll".into(),
            author: Some("Tenvy".into()),
            homepage: Some("https://example.invalid".into()),
            repository_url: Some("https://example.invalid/repo".into()),
            license: Some(PluginLicenseInfo {
                spdx_id: Some("MIT".into()),
                name: None,
                url: None,
            }),
            categories: vec!["control".into()],
            capabilities: vec!["capability.system-info.view".into()],
            telemetry: vec!["telemetry.system-info".into()],
            dependencies: vec!["core.system-info".into()],
            runtime: Some(PluginRuntimeDescriptor {
                r#type: Some(PluginRuntimeType::Native),
                sandboxed: Some(true),
                host: Some(PluginRuntimeHostContract {
                    api_version: Some("1.0.0".into()),
                    interfaces: vec!["tenvy.core/1".into()],
                }),
            }),
            requirements: PluginRequirements {
                min_agent_version: Some("1.0.0".into()),
                max_agent_version: None,
                min_client_version: Some("0.5.0".into()),
                platforms: vec![PluginPlatform::Windows],
                architectures: vec![PluginArchitecture::X86_64],
                required_modules: vec!["core.system-info".into()],
            },
            distribution: PluginDistribution {
                default_mode: PluginDeliveryMode::Automatic,
                auto_update: true,
                signature: PluginSignatureType::Sha256,
                signature_hash: Some("a".repeat(64)),
                signature_value: None,
                signature_signer: Some("Rootbay".into()),
                signature_timestamp: Some("2025-11-08T00:00:00Z".into()),
                signature_certificate_chain: vec!["Root CA".into()],
            },
            package_descriptor: PluginPackageDescriptor {
                artifact: "remote-desktop.zip".into(),
                size_bytes: Some(1024),
                hash: Some("b".repeat(64)),
            },
        }
    }

    #[test]
    fn validates_successfully() {
        let manifest = base_manifest();
        let ctx = context();
        assert!(validate_manifest(&manifest, &ctx).is_ok());
    }

    #[test]
    fn reports_multiple_errors() {
        let mut manifest = base_manifest();
        manifest.version = "1.0".into();
        manifest
            .requirements
            .required_modules
            .push("unknown".into());
        manifest.distribution.signature_hash = Some("123".into());
        manifest.package_descriptor.size_bytes = Some(-10);
        manifest.capabilities.push("".into());

        let ctx = context();
        let result = validate_manifest(&manifest, &ctx).unwrap_err();
        let messages: Vec<_> = result.errors().iter().map(|err| err.to_string()).collect();

        assert!(messages
            .iter()
            .any(|m| m.contains("invalid semantic version")));
        assert!(messages.iter().any(|m| m.contains("module `unknown`")));
        assert!(messages
            .iter()
            .any(|m| m.contains("expected 64-character hexadecimal string")));
        assert!(messages
            .iter()
            .any(|m| m.contains("size must be greater than zero")));
        assert!(messages
            .iter()
            .any(|m| m.contains("field `capabilities` is missing or blank")));
    }

    #[test]
    fn enforces_ed25519_signature_value() {
        let mut manifest = base_manifest();
        manifest.distribution.signature = PluginSignatureType::Ed25519;
        manifest.distribution.signature_hash = None;
        manifest.distribution.signature_value = Some("".into());

        let ctx = context();
        let result = validate_manifest(&manifest, &ctx).unwrap_err();
        assert!(result
            .errors()
            .iter()
            .any(|err| err.to_string().contains("distribution.signatureValue")));
    }
}
