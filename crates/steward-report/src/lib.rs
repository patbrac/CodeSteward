//! Public, serialization-safe report models.
//!
//! This crate is deliberately at the center of the workspace dependency graph. It
//! contains data contracts, not filesystem, process, network, or rendering logic.

#![forbid(unsafe_code)]

use serde::{Deserialize, Serialize};

/// Schema version emitted by the Phase 1 doctor report.
pub const DOCTOR_SCHEMA_VERSION: &str = "0.1.0";

/// A machine-readable report returned by `steward doctor`.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct DoctorReport {
    /// Version of this report shape.
    pub schema_version: String,
    /// Identity of the running executable.
    pub tool: ToolInfo,
    /// Non-semantic host information useful for installation diagnostics.
    pub host: HostInfo,
    /// True when every required health check passed.
    pub healthy: bool,
    /// Checks in stable presentation order.
    pub checks: Vec<DoctorCheck>,
}

/// Name and version of the running tool.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct ToolInfo {
    /// Stable executable name.
    pub name: String,
    /// Cargo package version.
    pub version: String,
}

/// Host attributes that may vary without changing semantic scanner results.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct HostInfo {
    /// Normalized operating-system family.
    pub os_family: OsFamily,
    /// Rust target architecture name.
    pub architecture: String,
}

/// Normalized operating-system families in the native-support contract.
#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum OsFamily {
    /// Microsoft Windows.
    Windows,
    /// Apple macOS.
    Macos,
    /// Linux distributions.
    Linux,
    /// A host outside the current native-support target families.
    Other,
}

impl OsFamily {
    /// Return the stable wire-format name.
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Windows => "windows",
            Self::Macos => "macos",
            Self::Linux => "linux",
            Self::Other => "other",
        }
    }
}

/// One deterministic installation or safety-boundary check.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct DoctorCheck {
    /// Stable, machine-readable check identifier.
    pub id: String,
    /// Check result.
    pub status: CheckStatus,
    /// Human-readable result with no control sequences.
    pub message: String,
}

/// Doctor check status.
#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum CheckStatus {
    /// A required check passed.
    Pass,
    /// An informational limitation that does not make the installation unhealthy.
    Warning,
    /// A required check failed.
    Fail,
}

impl CheckStatus {
    /// Return the stable wire-format name.
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Pass => "pass",
            Self::Warning => "warning",
            Self::Fail => "fail",
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{CheckStatus, OsFamily};

    #[test]
    fn enum_names_are_stable() {
        assert_eq!(OsFamily::Windows.as_str(), "windows");
        assert_eq!(OsFamily::Macos.as_str(), "macos");
        assert_eq!(OsFamily::Linux.as_str(), "linux");
        assert_eq!(OsFamily::Other.as_str(), "other");
        assert_eq!(CheckStatus::Pass.as_str(), "pass");
        assert_eq!(CheckStatus::Warning.as_str(), "warning");
        assert_eq!(CheckStatus::Fail.as_str(), "fail");
    }
}
