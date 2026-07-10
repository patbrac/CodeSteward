use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread;
use std::time::{Duration, Instant};

use serde_json::Value;
use steward_engine::MAX_TERMINAL_TEXT_BYTES;

static NEXT_TEMP: AtomicU64 = AtomicU64::new(0);

struct TempDirectory {
    path: PathBuf,
}

impl TempDirectory {
    fn new(label: &str) -> Self {
        let id = NEXT_TEMP.fetch_add(1, Ordering::Relaxed);
        let path =
            std::env::temp_dir().join(format!("steward-cli-{}-{id}-{label}", std::process::id()));
        fs::create_dir_all(&path).unwrap();
        Self { path }
    }

    fn path(&self) -> &Path {
        &self.path
    }
}

impl Drop for TempDirectory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.path);
    }
}

fn steward() -> Command {
    Command::new(env!("CARGO_BIN_EXE_steward"))
}

fn run(arguments: &[&str], current_dir: &Path) -> Output {
    steward()
        .args(arguments)
        .current_dir(current_dir)
        .output()
        .unwrap()
}

fn output_with_timeout(mut command: Command, timeout: Duration) -> Output {
    let mut child = command
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap();
    let started = Instant::now();
    loop {
        if child.try_wait().unwrap().is_some() {
            return child.wait_with_output().unwrap();
        }
        if started.elapsed() >= timeout {
            child.kill().unwrap();
            let output = child.wait_with_output().unwrap();
            panic!(
                "steward exceeded {timeout:?}; stdout={:?}; stderr={:?}",
                String::from_utf8_lossy(&output.stdout),
                String::from_utf8_lossy(&output.stderr)
            );
        }
        thread::sleep(Duration::from_millis(10));
    }
}

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../..")
}

fn stdout(output: &Output) -> String {
    String::from_utf8(output.stdout.clone()).unwrap()
}

fn stderr(output: &Output) -> String {
    String::from_utf8(output.stderr.clone()).unwrap()
}

#[test]
fn version_subcommand_has_stable_streams_and_exit() {
    let directory = TempDirectory::new("version");
    let output = run(&["version"], directory.path());
    assert_eq!(output.status.code(), Some(0));
    assert_eq!(
        stdout(&output),
        format!("steward {}\n", env!("CARGO_PKG_VERSION"))
    );
    assert!(stderr(&output).is_empty());
}

#[test]
fn doctor_supports_human_and_json_without_network_configuration() {
    let directory = TempDirectory::new("doctor");
    let human = run(&["doctor"], directory.path());
    assert_eq!(human.status.code(), Some(0));
    assert!(stdout(&human).contains("[pass] offline-default"));
    assert!(stdout(&human).contains("[pass] repository-non-execution"));
    assert!(!stdout(&human).contains('\u{1b}'));
    assert!(stderr(&human).is_empty());

    let json = run(&["doctor", "--format", "json"], directory.path());
    assert_eq!(json.status.code(), Some(0));
    let value: Value = serde_json::from_slice(&json.stdout).unwrap();
    assert_eq!(value["schema_version"], "0.1.0");
    assert_eq!(value["tool"]["name"], "steward");
    assert_eq!(value["healthy"], true);
    assert_eq!(value["checks"].as_array().unwrap().len(), 4);
    assert!(stderr(&json).is_empty());
}

#[test]
fn validates_crlf_config_at_a_unicode_path() {
    let directory = TempDirectory::new("unicode path");
    let path = directory.path().join("配置 file.yaml");
    fs::write(
        &path,
        b"version: \"0.1\"\r\nanalysis:\r\n  max_file_bytes: 2048\r\n",
    )
    .unwrap();

    let output = steward()
        .args(["config", "validate"])
        .arg(&path)
        .current_dir(directory.path())
        .output()
        .unwrap();
    assert_eq!(output.status.code(), Some(0));
    assert!(stdout(&output).contains("configuration version 0.1"));
    assert!(stdout(&output).contains("配置 file.yaml"));
    assert!(stderr(&output).is_empty());
}

#[test]
fn default_config_path_uses_the_native_current_directory() {
    let directory = TempDirectory::new("current directory");
    fs::write(directory.path().join("steward.yaml"), "version: \"0.1\"\n").unwrap();
    let output = run(&["config", "validate"], directory.path());
    assert_eq!(output.status.code(), Some(0));
    assert_eq!(
        stdout(&output),
        "valid: steward.yaml (configuration version 0.1)\n"
    );
}

#[test]
fn strict_unknown_keys_fail_and_compatibility_mode_reports_them() {
    let directory = TempDirectory::new("unknown");
    let path = directory.path().join("steward.yaml");
    fs::write(&path, "version: \"0.1\"\nanalysis:\n  future_limit: 2\n").unwrap();

    let strict = run(&["config", "validate"], directory.path());
    assert_eq!(strict.status.code(), Some(3));
    assert!(stdout(&strict).is_empty());
    assert!(stderr(&strict).contains("analysis.future_limit"));

    let compatible = run(&["config", "validate", "--allow-unknown"], directory.path());
    assert_eq!(compatible.status.code(), Some(0));
    assert!(stdout(&compatible).contains("ignored unknown key(s): analysis.future_limit"));
    assert!(stderr(&compatible).is_empty());
}

#[test]
fn missing_unsupported_and_non_utf8_configs_have_config_exit() {
    let directory = TempDirectory::new("invalid-config");
    for (name, bytes) in [
        ("missing.yaml", b"output:\n  format: terminal\n".as_slice()),
        ("version.yaml", b"version: \"9\"\n".as_slice()),
        ("encoding.yaml", &[0xff, 0xfe][..]),
    ] {
        let path = directory.path().join(name);
        fs::write(&path, bytes).unwrap();
        let output = steward()
            .args(["config", "validate"])
            .arg(path)
            .current_dir(directory.path())
            .output()
            .unwrap();
        assert_eq!(output.status.code(), Some(3), "fixture: {name}");
        assert!(stdout(&output).is_empty(), "fixture: {name}");
        assert!(!stderr(&output).is_empty(), "fixture: {name}");
    }
}

#[test]
fn missing_file_and_usage_have_distinct_stable_exits() {
    let directory = TempDirectory::new("exit-codes");
    let missing = run(
        &["config", "validate", "does-not-exist.yaml"],
        directory.path(),
    );
    assert_eq!(missing.status.code(), Some(4));
    assert!(stdout(&missing).is_empty());
    assert!(stderr(&missing).starts_with("error: cannot read configuration"));

    let usage = run(&["not-a-command"], directory.path());
    assert_eq!(usage.status.code(), Some(2));
    assert!(stdout(&usage).is_empty());
    assert!(stderr(&usage).contains("Usage:"));
}

#[test]
fn repository_command_fields_are_inert_and_rejected() {
    let directory = TempDirectory::new("non-execution");
    let marker = directory.path().join("must-not-exist");
    fs::write(
        directory.path().join("steward.yaml"),
        "version: \"0.1\"\ncommand: \"create-marker\"\n",
    )
    .unwrap();
    let output = run(&["config", "validate"], directory.path());
    assert_eq!(output.status.code(), Some(3));
    assert!(!marker.exists());
    assert!(stderr(&output).contains("unknown configuration key(s): command"));
}

#[test]
fn diagnostics_escape_terminal_control_characters() {
    let directory = TempDirectory::new("terminal-safety");
    let output = run(
        &["config", "validate", "bad\u{1b}[31m.yaml"],
        directory.path(),
    );
    assert_eq!(output.status.code(), Some(4));
    assert!(!stderr(&output).contains('\u{1b}'));
    assert!(stderr(&output).contains("\\u{001b}"));
}

#[test]
fn clap_diagnostics_escape_control_characters_from_invalid_arguments() {
    let directory = TempDirectory::new("clap-terminal-safety");
    let invalid_format = String::from("bad\u{1b}[31m\ninjected");
    let output = steward()
        .args(["doctor", "--format"])
        .arg(&invalid_format)
        .current_dir(directory.path())
        .output()
        .unwrap();

    assert_eq!(output.status.code(), Some(2));
    assert!(stdout(&output).is_empty());
    let diagnostic = stderr(&output);
    assert!(!diagnostic.contains('\u{1b}'));
    assert!(!diagnostic.contains("bad\u{1b}[31m\ninjected"));
    assert!(diagnostic.contains("bad\\u{001b}[31m\\ninjected"));

    let invalid_subcommand = String::from("bad\tcommand\u{1b}");
    let output = steward()
        .arg(&invalid_subcommand)
        .current_dir(directory.path())
        .output()
        .unwrap();
    assert_eq!(output.status.code(), Some(2));
    assert!(stdout(&output).is_empty());
    let diagnostic = stderr(&output);
    assert!(!diagnostic.contains('\u{1b}'));
    assert!(!diagnostic.contains("bad\tcommand\u{1b}"));
    assert!(diagnostic.contains("bad\\tcommand\\u{001b}"));
}

#[test]
fn help_and_version_keep_their_success_stream_contract() {
    let directory = TempDirectory::new("clap-success-streams");
    for arguments in [["--help"].as_slice(), ["--version"].as_slice()] {
        let output = run(arguments, directory.path());
        assert_eq!(output.status.code(), Some(0));
        assert!(!stdout(&output).is_empty());
        assert!(stderr(&output).is_empty());
    }
}

#[test]
fn invalid_config_diagnostics_do_not_echo_scalar_secrets() {
    const SYNTHETIC_SECRET: &str = "SYNTHETIC_CREDENTIAL_VALUE_DO_NOT_USE_123456789";
    let directory = TempDirectory::new("secret-safe-diagnostic");
    fs::write(
        directory.path().join("steward.yaml"),
        format!("version: \"0.1\"\nanalysis:\n  max_file_bytes: \"{SYNTHETIC_SECRET}\"\n"),
    )
    .unwrap();

    let output = run(&["config", "validate"], directory.path());
    assert_eq!(output.status.code(), Some(3));
    assert!(stdout(&output).is_empty());
    assert!(!stderr(&output).contains(SYNTHETIC_SECRET));
    assert!(stderr(&output).contains("analysis.max_file_bytes"));
}

#[test]
fn hostile_nested_config_is_rejected_within_a_bounded_cli_timeout() {
    let directory = TempDirectory::new("bounded-hostile-config");
    let depth = 500_000;
    let path = directory.path().join("deep.yaml");
    fs::write(
        &path,
        format!(
            "version: \"0.1\"\nfuture: {}null{}\n",
            "[".repeat(depth),
            "]".repeat(depth)
        ),
    )
    .unwrap();
    assert!(fs::metadata(&path).unwrap().len() < 1024 * 1024);

    for allow_unknown in [false, true] {
        let mut command = steward();
        command
            .args(["config", "validate"])
            .arg(&path)
            .current_dir(directory.path());
        if allow_unknown {
            command.arg("--allow-unknown");
        }
        let output = output_with_timeout(command, Duration::from_secs(5));
        assert_eq!(output.status.code(), Some(3));
        assert!(stdout(&output).is_empty());
        assert!(stderr(&output).contains("structural nesting depth"));
    }
}

#[test]
fn configuration_diagnostics_are_bounded() {
    let directory = TempDirectory::new("bounded-diagnostic");
    let long_unknown_key = "x".repeat(20_000);
    fs::write(
        directory.path().join("steward.yaml"),
        format!("version: \"0.1\"\n\"{long_unknown_key}\": true\n"),
    )
    .unwrap();

    let output = run(&["config", "validate"], directory.path());
    assert_eq!(output.status.code(), Some(3));
    assert!(stdout(&output).is_empty());
    let diagnostic = stderr(&output);
    assert!(diagnostic.len() <= "error: \n".len() + MAX_TERMINAL_TEXT_BYTES);
    assert!(!diagnostic.contains(&long_unknown_key));
}

#[test]
fn config_schema_json_examples_match_runtime_validation() {
    let directory = TempDirectory::new("schema-runtime-parity");
    for (kind, expected_exit) in [("valid", 0), ("invalid", 3)] {
        let examples = repository_root().join(format!("schemas/config/v0/examples/{kind}"));
        let mut paths = fs::read_dir(examples)
            .unwrap()
            .map(|entry| entry.unwrap().path())
            .collect::<Vec<_>>();
        paths.sort();
        assert!(!paths.is_empty());
        for path in paths {
            let output = steward()
                .args(["config", "validate"])
                .arg(&path)
                .current_dir(directory.path())
                .output()
                .unwrap();
            assert_eq!(
                output.status.code(),
                Some(expected_exit),
                "schema/runtime mismatch for {}: {}",
                path.display(),
                stderr(&output)
            );
        }
    }
}
