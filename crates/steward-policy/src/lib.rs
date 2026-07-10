//! Typed, declarative configuration primitives.
//!
//! Configuration is data only: this crate exposes no command, environment-variable,
//! plugin-loading, repository-execution, or network capability.

#![forbid(unsafe_code)]

use std::error::Error;
use std::fmt::{self, Display, Formatter};
use std::fs::File;
use std::io::{self, Read};
use std::path::{Path, PathBuf};
use std::str;

use serde::de::{self, Visitor};
use serde::{Deserialize, Deserializer, Serialize};

/// The only configuration contract supported by this release.
pub const CURRENT_CONFIG_VERSION: &str = "0.1";

/// Maximum accepted configuration size (one mebibyte).
pub const MAX_CONFIG_BYTES: usize = 1024 * 1024;

/// Maximum collection nesting accepted before YAML deserialization.
pub const MAX_CONFIG_NESTING_DEPTH: usize = 64;

/// Maximum structural tokens accepted before YAML deserialization.
pub const MAX_CONFIG_STRUCTURAL_TOKENS: usize = 65_536;

/// Default maximum source-file size for future analysis commands.
pub const DEFAULT_MAX_FILE_BYTES: u64 = 1024 * 1024;

/// Default maximum history depth for future analysis commands.
pub const DEFAULT_MAX_HISTORY_COMMITS: u64 = 10_000;

const MAX_FILE_BYTES_LIMIT: u64 = 1024 * 1024 * 1024;
const MAX_HISTORY_COMMITS_LIMIT: u64 = 1_000_000;

/// A fully validated, typed configuration.
#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
pub struct Config {
    /// Parsed configuration contract version.
    pub version: ConfigVersion,
    /// Bounded analysis settings.
    pub analysis: AnalysisConfig,
    /// Presentation defaults.
    pub output: OutputConfig,
    /// Initial typed policy settings.
    pub policy: PolicyConfig,
}

/// Supported configuration contract versions.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize)]
pub enum ConfigVersion {
    /// Experimental Phase 1 contract.
    #[serde(rename = "0.1")]
    V0_1,
}

impl ConfigVersion {
    /// Return the serialized version identifier.
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::V0_1 => CURRENT_CONFIG_VERSION,
        }
    }
}

/// Resource bounds shared by future analyzers.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(default)]
pub struct AnalysisConfig {
    /// Largest individual file that may be read for analysis.
    #[serde(deserialize_with = "deserialize_whole_number")]
    pub max_file_bytes: u64,
    /// Largest number of commits that may be traversed by default.
    #[serde(deserialize_with = "deserialize_whole_number")]
    pub max_history_commits: u64,
}

struct WholeNumberVisitor;

impl Visitor<'_> for WholeNumberVisitor {
    type Value = u64;

    fn expecting(&self, formatter: &mut Formatter<'_>) -> fmt::Result {
        formatter.write_str("an unquoted whole number")
    }

    fn visit_u64<E>(self, value: u64) -> Result<Self::Value, E> {
        Ok(value)
    }

    fn visit_i64<E>(self, value: i64) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        u64::try_from(value).map_err(|_| E::custom("must be a non-negative whole number"))
    }

    fn visit_f64<E>(self, value: f64) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        if value.is_finite() && value >= 0.0 && value <= u64::MAX as f64 && value.fract() == 0.0 {
            Ok(value as u64)
        } else {
            Err(E::custom("must be a non-negative whole number"))
        }
    }

    fn visit_str<E>(self, _value: &str) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        Err(E::custom("must be an unquoted whole number"))
    }
}

fn deserialize_whole_number<'de, D>(deserializer: D) -> Result<u64, D::Error>
where
    D: Deserializer<'de>,
{
    deserializer.deserialize_any(WholeNumberVisitor)
}

impl Default for AnalysisConfig {
    fn default() -> Self {
        Self {
            max_file_bytes: DEFAULT_MAX_FILE_BYTES,
            max_history_commits: DEFAULT_MAX_HISTORY_COMMITS,
        }
    }
}

/// Presentation defaults. They do not affect semantic findings.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(default)]
pub struct OutputConfig {
    /// Preferred renderer for future scan output.
    pub format: OutputFormat,
    /// Color behavior for terminal renderers.
    pub color: ColorChoice,
}

/// Supported output renderers in the Phase 1 configuration contract.
#[derive(Clone, Copy, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum OutputFormat {
    /// Human-readable terminal output.
    #[default]
    Terminal,
    /// Machine-readable JSON output.
    Json,
}

/// Terminal color preference.
#[derive(Clone, Copy, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ColorChoice {
    /// Detect whether output is attached to a terminal.
    #[default]
    Auto,
    /// Always emit color where the selected renderer supports it.
    Always,
    /// Never emit color.
    Never,
}

/// Initial typed policy settings.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(default)]
pub struct PolicyConfig {
    /// Policy mode. Phase 1 is intentionally advisory only.
    pub mode: PolicyMode,
}

/// Supported policy modes.
#[derive(Clone, Copy, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum PolicyMode {
    /// Findings never fail a command unless a later typed policy explicitly opts in.
    #[default]
    Advisory,
}

#[derive(Debug, Deserialize)]
struct RawConfig {
    version: yaml_serde::Value,
    #[serde(default)]
    analysis: AnalysisConfig,
    #[serde(default)]
    output: OutputConfig,
    #[serde(default)]
    policy: PolicyConfig,
}

/// Validation behavior selected explicitly by the caller.
#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct ValidationOptions {
    /// Ignore and report unknown keys for forward-compatible inspection.
    ///
    /// This does not permit an unsupported version or an invalid known value.
    pub allow_unknown_keys: bool,
}

/// Successful validation plus any explicitly ignored keys.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ValidatedConfig {
    /// Typed configuration with defaults applied.
    pub config: Config,
    /// Stable, sorted paths to unknown keys ignored by compatibility mode.
    pub ignored_keys: Vec<String>,
}

/// A configuration loading or validation error.
#[derive(Debug)]
pub enum ConfigError {
    /// The file could not be opened or read.
    Io {
        /// Config path supplied by the caller.
        path: PathBuf,
        /// Underlying operating-system error.
        source: io::Error,
    },
    /// The bounded reader observed more than [`MAX_CONFIG_BYTES`].
    TooLarge {
        /// Optional source path.
        path: Option<PathBuf>,
        /// Minimum observed size.
        observed_bytes: usize,
    },
    /// Configuration content was not valid UTF-8.
    InvalidUtf8 {
        /// Optional source path.
        path: Option<PathBuf>,
    },
    /// YAML structure exceeded a deterministic parser-safety limit.
    StructuralLimit {
        /// Stable name of the exceeded limit.
        limit: &'static str,
        /// Maximum accepted value.
        maximum: usize,
    },
    /// YAML/JSON syntax or a known field's type was invalid.
    Parse {
        /// Parser diagnostic.
        message: String,
    },
    /// More than one YAML document was supplied.
    MultipleDocuments,
    /// Strict mode observed unknown keys.
    UnknownKeys {
        /// Stable, sorted paths to unknown keys.
        keys: Vec<String>,
    },
    /// The mandatory version was syntactically valid but unsupported.
    UnsupportedVersion,
    /// Known values violated semantic resource bounds.
    InvalidValues {
        /// Stable list of actionable diagnostics.
        messages: Vec<String>,
    },
}

impl ConfigError {
    /// Whether the failure originated in filesystem I/O.
    #[must_use]
    pub const fn is_io(&self) -> bool {
        matches!(self, Self::Io { .. })
    }
}

impl Display for ConfigError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> fmt::Result {
        match self {
            Self::Io { path, source } => {
                write!(
                    formatter,
                    "cannot read configuration {}: {source}",
                    path.display()
                )
            }
            Self::TooLarge {
                path,
                observed_bytes,
            } => {
                if let Some(path) = path {
                    write!(formatter, "configuration {} is too large", path.display())?;
                } else {
                    formatter.write_str("configuration is too large")?;
                }
                write!(
                    formatter,
                    ": observed at least {observed_bytes} bytes; limit is {MAX_CONFIG_BYTES}"
                )
            }
            Self::InvalidUtf8 { path } => {
                if let Some(path) = path {
                    write!(
                        formatter,
                        "configuration {} is not valid UTF-8",
                        path.display()
                    )
                } else {
                    formatter.write_str("configuration is not valid UTF-8")
                }
            }
            Self::StructuralLimit { limit, maximum } => write!(
                formatter,
                "invalid configuration: structural {limit} exceeds the limit of {maximum}"
            ),
            Self::Parse { message } => write!(formatter, "invalid configuration: {message}"),
            Self::MultipleDocuments => {
                formatter.write_str("invalid configuration: exactly one YAML document is allowed")
            }
            Self::UnknownKeys { keys } => write!(
                formatter,
                "unknown configuration key(s): {}; use --allow-unknown only for forward-compatible inspection",
                keys.join(", ")
            ),
            Self::UnsupportedVersion => write!(
                formatter,
                "unsupported configuration version; supported version is {CURRENT_CONFIG_VERSION:?}"
            ),
            Self::InvalidValues { messages } => {
                write!(
                    formatter,
                    "invalid configuration value(s): {}",
                    messages.join("; ")
                )
            }
        }
    }
}

impl Error for ConfigError {
    fn source(&self) -> Option<&(dyn Error + 'static)> {
        match self {
            Self::Io { source, .. } => Some(source),
            _ => None,
        }
    }
}

/// Load and validate a UTF-8 YAML or JSON configuration from `path`.
///
/// Reading is capped before allocation can grow beyond the documented limit.
pub fn load_config(
    path: &Path,
    options: ValidationOptions,
) -> Result<ValidatedConfig, ConfigError> {
    let file = File::open(path).map_err(|source| ConfigError::Io {
        path: path.to_path_buf(),
        source,
    })?;

    let mut bytes = Vec::with_capacity(8 * 1024);
    file.take((MAX_CONFIG_BYTES + 1) as u64)
        .read_to_end(&mut bytes)
        .map_err(|source| ConfigError::Io {
            path: path.to_path_buf(),
            source,
        })?;

    if bytes.len() > MAX_CONFIG_BYTES {
        return Err(ConfigError::TooLarge {
            path: Some(path.to_path_buf()),
            observed_bytes: bytes.len(),
        });
    }

    let text = str::from_utf8(&bytes).map_err(|_| ConfigError::InvalidUtf8 {
        path: Some(path.to_path_buf()),
    })?;
    validate_config(text, options)
}

/// Validate one UTF-8 YAML or JSON configuration document.
pub fn validate_config(
    text: &str,
    options: ValidationOptions,
) -> Result<ValidatedConfig, ConfigError> {
    if text.len() > MAX_CONFIG_BYTES {
        return Err(ConfigError::TooLarge {
            path: None,
            observed_bytes: text.len(),
        });
    }

    let text = text.strip_prefix('\u{feff}').unwrap_or(text);
    validate_structural_complexity(text)?;
    let mut documents = yaml_serde::Deserializer::from_str(text);
    let document = documents.next().ok_or_else(|| ConfigError::Parse {
        message: "the document is empty; mandatory field `version` is missing".to_owned(),
    })?;

    let mut ignored_keys = Vec::new();
    let raw: RawConfig = serde_ignored::deserialize(document, |path| {
        ignored_keys.push(path.to_string());
    })
    .map_err(|error| ConfigError::Parse {
        message: safe_yaml_diagnostic(&error),
    })?;

    if documents.next().is_some() {
        return Err(ConfigError::MultipleDocuments);
    }

    ignored_keys.sort_unstable();
    ignored_keys.dedup();
    if !options.allow_unknown_keys && !ignored_keys.is_empty() {
        return Err(ConfigError::UnknownKeys { keys: ignored_keys });
    }

    let version_text = match raw.version {
        yaml_serde::Value::String(version) => version,
        _ => {
            return Err(ConfigError::Parse {
                message: "field `version` must be a quoted string".to_owned(),
            });
        }
    };
    let version = match version_text.as_str() {
        CURRENT_CONFIG_VERSION => ConfigVersion::V0_1,
        _ => {
            return Err(ConfigError::UnsupportedVersion);
        }
    };

    let mut messages = Vec::new();
    if !(1..=MAX_FILE_BYTES_LIMIT).contains(&raw.analysis.max_file_bytes) {
        messages.push(format!(
            "analysis.max_file_bytes must be between 1 and {MAX_FILE_BYTES_LIMIT}"
        ));
    }
    if !(1..=MAX_HISTORY_COMMITS_LIMIT).contains(&raw.analysis.max_history_commits) {
        messages.push(format!(
            "analysis.max_history_commits must be between 1 and {MAX_HISTORY_COMMITS_LIMIT}"
        ));
    }
    if !messages.is_empty() {
        return Err(ConfigError::InvalidValues { messages });
    }

    Ok(ValidatedConfig {
        config: Config {
            version,
            analysis: raw.analysis,
            output: raw.output,
            policy: raw.policy,
        },
        ignored_keys,
    })
}

fn validate_structural_complexity(text: &str) -> Result<(), ConfigError> {
    let mut indentation = Vec::new();
    let mut flow_collections = Vec::new();
    let mut quote = None;
    let mut double_quote_escape = false;
    let mut block_scalar_parent = None;
    let mut tokens = 0_usize;

    for raw_line in text.split('\n') {
        let line = raw_line.strip_suffix('\r').unwrap_or(raw_line);
        let indent = line.bytes().take_while(|byte| *byte == b' ').count();
        let trimmed = line.trim();

        if let Some(parent) = block_scalar_parent {
            if trimmed.is_empty() || indent > parent {
                continue;
            }
            block_scalar_parent = None;
        }

        if quote.is_none() && (trimmed.is_empty() || line.trim_start().starts_with('#')) {
            continue;
        }

        if quote.is_none() && flow_collections.is_empty() {
            while indentation.last().is_some_and(|level| *level >= indent) {
                indentation.pop();
            }
            indentation.push(indent);
            check_nesting_depth(indentation.len(), flow_collections.len())?;
            add_structural_token(&mut tokens)?;
        }

        let mut characters = line.char_indices().peekable();
        let mut previous = None;
        let mut at_content_start = true;
        let mut expecting_value = false;

        while let Some((index, character)) = characters.next() {
            match quote {
                Some('\'') => {
                    if character == '\'' {
                        if characters.peek().is_some_and(|(_, next)| *next == '\'') {
                            characters.next();
                        } else {
                            quote = None;
                        }
                    }
                    continue;
                }
                Some('"') => {
                    if double_quote_escape {
                        double_quote_escape = false;
                    } else if character == '\\' {
                        double_quote_escape = true;
                    } else if character == '"' {
                        quote = None;
                    }
                    continue;
                }
                Some(_) => unreachable!("quote state is limited to YAML quote characters"),
                None => {}
            }

            if character == '#' && previous.is_none_or(char::is_whitespace) {
                break;
            }
            if character.is_whitespace() {
                previous = Some(character);
                continue;
            }

            match character {
                '\'' | '"' => {
                    quote = Some(character);
                    expecting_value = false;
                }
                ':' => {
                    add_structural_token(&mut tokens)?;
                    expecting_value = true;
                }
                '-' if at_content_start
                    && characters
                        .peek()
                        .is_none_or(|(_, next)| char::is_whitespace(*next)) =>
                {
                    add_structural_token(&mut tokens)?;
                    expecting_value = true;
                }
                '|' | '>' if expecting_value && is_block_scalar_header(&line[index + 1..]) => {
                    block_scalar_parent = Some(indent);
                    break;
                }
                '[' | '{' => {
                    add_structural_token(&mut tokens)?;
                    flow_collections.push(character);
                    check_nesting_depth(indentation.len(), flow_collections.len())?;
                    expecting_value = false;
                }
                ']' | '}' => {
                    add_structural_token(&mut tokens)?;
                    flow_collections.pop();
                    expecting_value = false;
                }
                ',' => {
                    add_structural_token(&mut tokens)?;
                    expecting_value = false;
                }
                _ => expecting_value = false,
            }
            at_content_start = false;
            previous = Some(character);
        }

        double_quote_escape = false;
    }

    Ok(())
}

fn check_nesting_depth(indentation: usize, flow_collections: usize) -> Result<(), ConfigError> {
    let nested_indentation = indentation.saturating_sub(1);
    if nested_indentation.saturating_add(flow_collections) > MAX_CONFIG_NESTING_DEPTH {
        return Err(ConfigError::StructuralLimit {
            limit: "nesting depth",
            maximum: MAX_CONFIG_NESTING_DEPTH,
        });
    }
    Ok(())
}

fn add_structural_token(tokens: &mut usize) -> Result<(), ConfigError> {
    *tokens = tokens.saturating_add(1);
    if *tokens > MAX_CONFIG_STRUCTURAL_TOKENS {
        return Err(ConfigError::StructuralLimit {
            limit: "token count",
            maximum: MAX_CONFIG_STRUCTURAL_TOKENS,
        });
    }
    Ok(())
}

fn is_block_scalar_header(suffix: &str) -> bool {
    let indicators = suffix
        .split_once('#')
        .map_or(suffix, |(before_comment, _)| before_comment)
        .trim();
    indicators
        .chars()
        .all(|character| matches!(character, '+' | '-' | '0'..='9'))
}

fn safe_yaml_diagnostic(error: &yaml_serde::Error) -> String {
    const KNOWN_PATHS: &[&str] = &[
        "analysis.max_file_bytes",
        "analysis.max_history_commits",
        "output.format",
        "output.color",
        "policy.mode",
        "analysis",
        "output",
        "policy",
        "version",
    ];

    let raw = error.to_string();
    let path = KNOWN_PATHS.iter().find(|path| {
        raw.strip_prefix(**path)
            .is_some_and(|rest| rest.starts_with(": "))
    });
    let issue = if raw.contains("missing field `version`") {
        "mandatory field `version` is missing".to_owned()
    } else if raw.contains("duplicate field") {
        "a configuration field is duplicated".to_owned()
    } else if raw.contains("unknown variant") {
        path.map_or_else(
            || "a configuration field has an unsupported value".to_owned(),
            |path| format!("field `{path}` has an unsupported value"),
        )
    } else if raw.contains("whole number") {
        path.map_or_else(
            || "a configuration field must be an unquoted whole number".to_owned(),
            |path| format!("field `{path}` must be an unquoted whole number"),
        )
    } else if raw.contains("invalid type") {
        path.map_or_else(
            || "a configuration field has the wrong type".to_owned(),
            |path| format!("field `{path}` has the wrong type"),
        )
    } else if raw.contains("recursion limit exceeded") {
        "configuration nesting exceeds the YAML deserializer limit".to_owned()
    } else if raw.contains("repetition limit exceeded") {
        "configuration aliases exceed the YAML deserializer limit".to_owned()
    } else {
        "YAML syntax or structure is invalid".to_owned()
    };

    error.location().map_or(issue.clone(), |location| {
        format!(
            "{issue} at line {}, column {}",
            location.line(),
            location.column()
        )
    })
}

#[cfg(test)]
mod tests {
    use super::{
        AnalysisConfig, ConfigError, ConfigVersion, MAX_CONFIG_BYTES, MAX_CONFIG_NESTING_DEPTH,
        MAX_CONFIG_STRUCTURAL_TOKENS, OutputFormat, ValidationOptions, add_structural_token,
        validate_config,
    };

    const MINIMAL: &str = "version: \"0.1\"\n";

    #[test]
    fn minimal_config_applies_typed_defaults() {
        let validated = validate_config(MINIMAL, ValidationOptions::default()).unwrap();
        assert_eq!(validated.config.version, ConfigVersion::V0_1);
        assert_eq!(validated.config.analysis, AnalysisConfig::default());
        assert_eq!(validated.config.output.format, OutputFormat::Terminal);
        assert!(validated.ignored_keys.is_empty());
    }

    #[test]
    fn crlf_and_lf_are_semantically_equal() {
        let lf = "version: \"0.1\"\nanalysis:\n  max_file_bytes: 2048\n";
        let crlf = lf.replace('\n', "\r\n");
        let left = validate_config(lf, ValidationOptions::default()).unwrap();
        let right = validate_config(&crlf, ValidationOptions::default()).unwrap();
        assert_eq!(left.config, right.config);
    }

    #[test]
    fn utf8_bom_is_accepted() {
        let validated =
            validate_config("\u{feff}version: \"0.1\"\n", ValidationOptions::default()).unwrap();
        assert_eq!(validated.config.version.as_str(), "0.1");
    }

    #[test]
    fn nested_unknown_keys_are_errors_by_default() {
        let error = validate_config(
            "version: \"0.1\"\nanalysis:\n  mystery_limit: 5\nunknown_root: true\n",
            ValidationOptions::default(),
        )
        .unwrap_err();
        match error {
            ConfigError::UnknownKeys { keys } => {
                assert_eq!(keys, vec!["analysis.mystery_limit", "unknown_root"]);
            }
            other => panic!("unexpected error: {other}"),
        }
    }

    #[test]
    fn compatibility_mode_reports_unknown_keys_but_keeps_known_values() {
        let validated = validate_config(
            "version: \"0.1\"\nanalysis:\n  max_file_bytes: 2048\n  future: true\n",
            ValidationOptions {
                allow_unknown_keys: true,
            },
        )
        .unwrap();
        assert_eq!(validated.config.analysis.max_file_bytes, 2048);
        assert_eq!(validated.ignored_keys, vec!["analysis.future"]);
    }

    #[test]
    fn compatibility_mode_does_not_accept_unsupported_versions() {
        let error = validate_config(
            "version: \"9\"\nfuture: true\n",
            ValidationOptions {
                allow_unknown_keys: true,
            },
        )
        .unwrap_err();
        assert!(matches!(error, ConfigError::UnsupportedVersion));
    }

    #[test]
    fn resource_bounds_are_checked_after_deserialization() {
        let error = validate_config(
            "version: \"0.1\"\nanalysis:\n  max_file_bytes: 0\n  max_history_commits: 1000001\n",
            ValidationOptions::default(),
        )
        .unwrap_err();
        match error {
            ConfigError::InvalidValues { messages } => assert_eq!(messages.len(), 2),
            other => panic!("unexpected error: {other}"),
        }
    }

    #[test]
    fn multiple_yaml_documents_are_rejected() {
        let error = validate_config(
            "version: \"0.1\"\n---\nversion: \"0.1\"\n",
            ValidationOptions::default(),
        )
        .unwrap_err();
        assert!(matches!(error, ConfigError::MultipleDocuments));
    }

    #[test]
    fn non_string_version_is_rejected() {
        let error = validate_config("version: 0.1\n", ValidationOptions::default()).unwrap_err();
        assert!(matches!(error, ConfigError::Parse { .. }));
    }

    #[test]
    fn numeric_fields_do_not_coerce_strings() {
        let error = validate_config(
            "version: \"0.1\"\nanalysis:\n  max_file_bytes: \"2048\"\n",
            ValidationOptions::default(),
        )
        .unwrap_err();
        assert!(matches!(error, ConfigError::Parse { .. }));
    }

    #[test]
    fn duplicate_known_keys_are_rejected() {
        let error = validate_config(
            "version: \"0.1\"\nversion: \"0.1\"\n",
            ValidationOptions::default(),
        )
        .unwrap_err();
        assert!(matches!(error, ConfigError::Parse { .. }));
    }

    #[test]
    fn oversized_in_memory_config_is_rejected_before_parsing() {
        let input = " ".repeat(MAX_CONFIG_BYTES + 1);
        let error = validate_config(&input, ValidationOptions::default()).unwrap_err();
        assert!(matches!(error, ConfigError::TooLarge { .. }));
    }

    #[test]
    fn structural_nesting_limit_has_an_exact_boundary_in_both_modes() {
        let at_limit = format!(
            "version: \"0.1\"\nfuture: {}null{}\n",
            "[".repeat(MAX_CONFIG_NESTING_DEPTH),
            "]".repeat(MAX_CONFIG_NESTING_DEPTH)
        );
        assert!(
            validate_config(
                &at_limit,
                ValidationOptions {
                    allow_unknown_keys: true,
                },
            )
            .is_ok()
        );

        let above_limit = format!(
            "version: \"0.1\"\nfuture: {}null{}\n",
            "[".repeat(MAX_CONFIG_NESTING_DEPTH + 1),
            "]".repeat(MAX_CONFIG_NESTING_DEPTH + 1)
        );

        for allow_unknown_keys in [false, true] {
            let error = validate_config(&above_limit, ValidationOptions { allow_unknown_keys })
                .unwrap_err();
            assert!(matches!(
                error,
                ConfigError::StructuralLimit {
                    limit: "nesting depth",
                    maximum: MAX_CONFIG_NESTING_DEPTH,
                }
            ));
        }
    }

    #[test]
    fn structural_token_limit_accepts_the_limit_and_rejects_the_next_token() {
        let mut tokens = MAX_CONFIG_STRUCTURAL_TOKENS - 1;
        add_structural_token(&mut tokens).unwrap();
        assert_eq!(tokens, MAX_CONFIG_STRUCTURAL_TOKENS);

        let error = add_structural_token(&mut tokens).unwrap_err();
        assert!(matches!(
            error,
            ConfigError::StructuralLimit {
                limit: "token count",
                maximum: MAX_CONFIG_STRUCTURAL_TOKENS,
            }
        ));
    }

    #[test]
    fn very_deep_flow_input_is_rejected_before_the_yaml_loader_can_expand_work() {
        let depth = 50_000;
        let input = format!(
            "version: \"0.1\"\nfuture: {}null{}\n",
            "[".repeat(depth),
            "]".repeat(depth)
        );
        let error = validate_config(
            &input,
            ValidationOptions {
                allow_unknown_keys: true,
            },
        )
        .unwrap_err();
        assert!(matches!(error, ConfigError::StructuralLimit { .. }));
    }

    #[test]
    fn flat_structural_token_flood_is_rejected() {
        let input = format!(
            "version: \"0.1\"\nfuture: [{}]\n",
            "0,".repeat(MAX_CONFIG_STRUCTURAL_TOKENS)
        );
        let error = validate_config(
            &input,
            ValidationOptions {
                allow_unknown_keys: true,
            },
        )
        .unwrap_err();
        assert!(matches!(
            error,
            ConfigError::StructuralLimit {
                limit: "token count",
                maximum: MAX_CONFIG_STRUCTURAL_TOKENS,
            }
        ));
    }

    #[test]
    fn structural_characters_inside_scalars_do_not_consume_the_budget() {
        let brackets = "[".repeat(MAX_CONFIG_NESTING_DEPTH + 1);
        let quoted = format!("version: \"0.1\"\nfuture: {brackets:?}\n");
        let block = format!("version: \"0.1\"\nfuture: |\n  {brackets}\n");
        let options = ValidationOptions {
            allow_unknown_keys: true,
        };
        assert!(validate_config(&quoted, options).is_ok());
        assert!(validate_config(&block, options).is_ok());
    }

    #[test]
    fn integral_json_number_representations_match_json_schema_integer_semantics() {
        let validated = validate_config(
            r#"{"version":"0.1","analysis":{"max_file_bytes":1.0,"max_history_commits":1e3}}"#,
            ValidationOptions::default(),
        )
        .unwrap();
        assert_eq!(validated.config.analysis.max_file_bytes, 1);
        assert_eq!(validated.config.analysis.max_history_commits, 1_000);

        for invalid in ["1.5", "\"1\""] {
            let input = format!(r#"{{"version":"0.1","analysis":{{"max_file_bytes":{invalid}}}}}"#);
            assert!(validate_config(&input, ValidationOptions::default()).is_err());
        }
    }

    #[test]
    fn diagnostics_do_not_echo_invalid_scalar_values_or_unsupported_versions() {
        const SYNTHETIC_SECRET: &str = "SYNTHETIC_CREDENTIAL_VALUE_DO_NOT_USE_123456789";
        for input in [
            format!("version: \"0.1\"\nanalysis:\n  max_file_bytes: \"{SYNTHETIC_SECRET}\"\n"),
            format!("version: \"{SYNTHETIC_SECRET}\"\n"),
            format!("version: \"0.1\"\noutput:\n  format: \"{SYNTHETIC_SECRET}\"\n"),
            format!("version: \"0.1\"\nfuture: \"{SYNTHETIC_SECRET}\"\n"),
        ] {
            let error = validate_config(&input, ValidationOptions::default()).unwrap_err();
            for diagnostic in [error.to_string(), format!("{error:?}")] {
                assert!(!diagnostic.contains(SYNTHETIC_SECRET));
            }
        }
    }
}
