//! Reusable Phase 1 engine APIs.
//!
//! The binary crate is intentionally a thin adapter over this crate. Nothing in
//! this crate initializes a network client or executes a repository-controlled
//! process.

#![forbid(unsafe_code)]

use std::fs;
use std::path::{Path, PathBuf};

use steward_policy::load_config;
pub use steward_policy::{CURRENT_CONFIG_VERSION, ConfigError, ValidatedConfig, ValidationOptions};
use steward_report::{
    CheckStatus, DOCTOR_SCHEMA_VERSION, DoctorCheck, DoctorReport, HostInfo, OsFamily, ToolInfo,
};

/// Stable executable name.
pub const TOOL_NAME: &str = "steward";

/// Version of the reusable engine and CLI.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

/// Maximum UTF-8 bytes emitted for one sanitized terminal-controlled value.
pub const MAX_TERMINAL_TEXT_BYTES: usize = 8 * 1024;

/// Stable process exit codes used by every CLI adapter.
pub mod exit {
    /// Command completed successfully.
    pub const SUCCESS: u8 = 0;
    /// Command-line syntax or usage was invalid.
    pub const USAGE: u8 = 2;
    /// Configuration syntax, shape, version, or value was invalid.
    pub const CONFIG_INVALID: u8 = 3;
    /// A requested file could not be opened or read.
    pub const IO: u8 = 4;
    /// An unexpected internal invariant failed.
    pub const INTERNAL: u8 = 70;
}

/// Inputs to the reusable doctor operation.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct DoctorOptions {
    /// Explicit configuration path. Without one, `steward.yaml` is inspected only
    /// when it exists in the current directory.
    pub config_path: Option<PathBuf>,
}

/// Doctor report plus the process outcome an adapter should return.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DoctorOutcome {
    /// Machine-readable report.
    pub report: DoctorReport,
    /// Stable process exit code.
    pub exit_code: u8,
}

/// Return the stable human-readable version line.
#[must_use]
pub fn version_line() -> String {
    format!("{TOOL_NAME} {VERSION}")
}

/// Validate a configuration file through the shared policy implementation.
pub fn validate_config_path(
    path: &Path,
    options: ValidationOptions,
) -> Result<ValidatedConfig, ConfigError> {
    load_config(path, options)
}

/// Inspect the installation and the Phase 1 security defaults without network
/// access or child-process execution.
#[must_use]
pub fn doctor(options: &DoctorOptions) -> DoctorOutcome {
    let host = HostInfo {
        os_family: current_os_family(),
        architecture: std::env::consts::ARCH.to_owned(),
    };

    let mut checks = vec![
        DoctorCheck {
            id: "offline-default".to_owned(),
            status: CheckStatus::Pass,
            message: "Phase 1 commands initialize no network client".to_owned(),
        },
        DoctorCheck {
            id: "repository-non-execution".to_owned(),
            status: CheckStatus::Pass,
            message: "Phase 1 commands execute no repository-controlled code".to_owned(),
        },
    ];

    checks.push(match host.os_family {
        OsFamily::Windows | OsFamily::Macos | OsFamily::Linux => DoctorCheck {
            id: "native-host-family".to_owned(),
            status: CheckStatus::Pass,
            message: format!(
                "{} is a first-class native target family",
                host.os_family.as_str()
            ),
        },
        OsFamily::Other => DoctorCheck {
            id: "native-host-family".to_owned(),
            status: CheckStatus::Warning,
            message: "host is outside the current Windows, macOS, and Linux target families"
                .to_owned(),
        },
    });

    let (config_check, config_exit) = inspect_config(options.config_path.as_deref());
    checks.push(config_check);

    let healthy = checks.iter().all(|check| check.status != CheckStatus::Fail);
    DoctorOutcome {
        report: DoctorReport {
            schema_version: DOCTOR_SCHEMA_VERSION.to_owned(),
            tool: ToolInfo {
                name: TOOL_NAME.to_owned(),
                version: VERSION.to_owned(),
            },
            host,
            healthy,
            checks,
        },
        exit_code: config_exit,
    }
}

/// Render a doctor report without terminal control sequences.
#[must_use]
pub fn render_doctor_human(report: &DoctorReport) -> String {
    let mut rendered = String::new();
    rendered.push_str("steward doctor\n");
    rendered.push_str(&format!("version: {}\n", report.tool.version));
    rendered.push_str(&format!(
        "host: {}/{}\n",
        report.host.os_family.as_str(),
        sanitize_terminal_text(&report.host.architecture)
    ));
    rendered.push_str(if report.healthy {
        "overall: healthy\n"
    } else {
        "overall: unhealthy\n"
    });
    for check in &report.checks {
        rendered.push_str(&format!(
            "[{}] {}: {}\n",
            check.status.as_str(),
            sanitize_terminal_text(&check.id),
            sanitize_terminal_text(&check.message)
        ));
    }
    rendered
}

/// Escape terminal control characters while preserving ordinary Unicode text.
#[must_use]
pub fn sanitize_terminal_text(input: &str) -> String {
    let mut output = String::with_capacity(input.len().min(MAX_TERMINAL_TEXT_BYTES));
    let mut characters = input.chars().peekable();
    while let Some(character) = characters.next() {
        let has_more = characters.peek().is_some();
        match character {
            '\n' => {
                if !push_terminal_fragment(&mut output, "\\n", has_more) {
                    break;
                }
            }
            '\r' => {
                if !push_terminal_fragment(&mut output, "\\r", has_more) {
                    break;
                }
            }
            '\t' => {
                if !push_terminal_fragment(&mut output, "\\t", has_more) {
                    break;
                }
            }
            character if character.is_control() => {
                let escaped = format!("\\u{{{:04x}}}", u32::from(character));
                if !push_terminal_fragment(&mut output, &escaped, has_more) {
                    break;
                }
            }
            character => {
                let mut bytes = [0_u8; 4];
                let encoded = character.encode_utf8(&mut bytes);
                if !push_terminal_fragment(&mut output, encoded, has_more) {
                    break;
                }
            }
        }
    }
    output
}

fn push_terminal_fragment(output: &mut String, fragment: &str, has_more: bool) -> bool {
    const TRUNCATION_MARKER: &str = "...";
    let reserved = if has_more { TRUNCATION_MARKER.len() } else { 0 };
    if output
        .len()
        .saturating_add(fragment.len())
        .saturating_add(reserved)
        > MAX_TERMINAL_TEXT_BYTES
    {
        output.push_str(TRUNCATION_MARKER);
        false
    } else {
        output.push_str(fragment);
        true
    }
}

fn current_os_family() -> OsFamily {
    if cfg!(target_os = "windows") {
        OsFamily::Windows
    } else if cfg!(target_os = "macos") {
        OsFamily::Macos
    } else if cfg!(target_os = "linux") {
        OsFamily::Linux
    } else {
        OsFamily::Other
    }
}

fn inspect_config(explicit_path: Option<&Path>) -> (DoctorCheck, u8) {
    let path = explicit_path.unwrap_or_else(|| Path::new("steward.yaml"));

    if explicit_path.is_none() {
        match fs::metadata(path) {
            Ok(_) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                return (
                    DoctorCheck {
                        id: "configuration".to_owned(),
                        status: CheckStatus::Pass,
                        message: "no steward.yaml found; built-in defaults are available"
                            .to_owned(),
                    },
                    exit::SUCCESS,
                );
            }
            Err(error) => {
                return (
                    DoctorCheck {
                        id: "configuration".to_owned(),
                        status: CheckStatus::Fail,
                        message: sanitize_terminal_text(&format!(
                            "cannot inspect {}: {error}",
                            path.display()
                        )),
                    },
                    exit::IO,
                );
            }
        }
    }

    match load_config(path, ValidationOptions::default()) {
        Ok(validated) => (
            DoctorCheck {
                id: "configuration".to_owned(),
                status: CheckStatus::Pass,
                message: sanitize_terminal_text(&format!(
                    "{} is valid configuration version {}",
                    path.display(),
                    validated.config.version.as_str()
                )),
            },
            exit::SUCCESS,
        ),
        Err(error) => {
            let exit_code = if error.is_io() {
                exit::IO
            } else {
                exit::CONFIG_INVALID
            };
            (
                DoctorCheck {
                    id: "configuration".to_owned(),
                    status: CheckStatus::Fail,
                    message: sanitize_terminal_text(&error.to_string()),
                },
                exit_code,
            )
        }
    }
}

#[cfg(test)]
mod tests {
    use std::fs;
    use std::path::PathBuf;
    use std::sync::atomic::{AtomicU64, Ordering};

    use super::{
        DoctorOptions, doctor, exit, render_doctor_human, sanitize_terminal_text, version_line,
    };

    static NEXT_TEMP: AtomicU64 = AtomicU64::new(0);

    fn temp_path(name: &str) -> PathBuf {
        let id = NEXT_TEMP.fetch_add(1, Ordering::Relaxed);
        std::env::temp_dir().join(format!("steward-engine-{}-{id}-{name}", std::process::id()))
    }

    #[test]
    fn version_line_is_stable() {
        assert_eq!(
            version_line(),
            format!("steward {}", env!("CARGO_PKG_VERSION"))
        );
    }

    #[test]
    fn doctor_has_fixed_security_checks_and_no_control_sequences() {
        let missing = temp_path("missing.yaml");
        let outcome = doctor(&DoctorOptions {
            config_path: Some(missing),
        });
        assert_eq!(outcome.exit_code, exit::IO);
        assert!(!outcome.report.healthy);
        assert_eq!(outcome.report.checks[0].id, "offline-default");
        assert_eq!(outcome.report.checks[1].id, "repository-non-execution");
        assert!(!render_doctor_human(&outcome.report).contains('\u{1b}'));
    }

    #[test]
    fn doctor_validates_an_explicit_unicode_config_path() {
        let path = temp_path("配置 file.yaml");
        fs::write(&path, "version: \"0.1\"\r\n").unwrap();
        let outcome = doctor(&DoctorOptions {
            config_path: Some(path.clone()),
        });
        fs::remove_file(path).unwrap();
        assert_eq!(outcome.exit_code, exit::SUCCESS);
        assert!(outcome.report.healthy);
    }

    #[test]
    fn terminal_sanitization_preserves_unicode_and_escapes_controls() {
        assert_eq!(
            sanitize_terminal_text("配置\n\u{1b}[31m"),
            "配置\\n\\u{001b}[31m"
        );
    }

    #[test]
    fn terminal_sanitization_bounds_diagnostic_output() {
        let rendered = sanitize_terminal_text(&"x".repeat(100_000));
        assert!(rendered.len() <= 8_192);
        assert!(rendered.ends_with("..."));
    }
}
