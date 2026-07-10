//! Thin command-line adapter over `steward-engine`.

#![forbid(unsafe_code)]

use std::ffi::{OsStr, OsString};
use std::io::{self, Write};
use std::path::PathBuf;
use std::process::ExitCode;

use clap::{Parser, Subcommand, ValueEnum};
use steward_engine::{
    DoctorOptions, ValidationOptions, doctor, exit, render_doctor_human, sanitize_terminal_text,
    validate_config_path, version_line,
};

#[derive(Debug, Parser)]
#[command(
    name = "steward",
    version,
    about = "Deterministic, offline-first maintenance intelligence",
    disable_help_subcommand = true
)]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    /// Print the executable version.
    Version,
    /// Check installation, platform, safety defaults, and optional configuration.
    Doctor {
        /// Output representation.
        #[arg(long, value_enum, default_value_t = DoctorFormat::Human)]
        format: DoctorFormat,
        /// Validate this file instead of discovering ./steward.yaml.
        #[arg(long, value_name = "PATH")]
        config: Option<PathBuf>,
    },
    /// Inspect and validate declarative configuration.
    Config {
        #[command(subcommand)]
        command: ConfigCommand,
    },
}

#[derive(Debug, Subcommand)]
enum ConfigCommand {
    /// Validate one steward.yaml file without executing its contents.
    Validate {
        /// Configuration path (defaults to ./steward.yaml).
        #[arg(default_value = "steward.yaml", value_name = "PATH")]
        path: PathBuf,
        /// Ignore and report unknown keys; known values and version remain strict.
        #[arg(long)]
        allow_unknown: bool,
    },
}

#[derive(Clone, Copy, Debug, Default, ValueEnum)]
enum DoctorFormat {
    /// Stable human-readable text.
    #[default]
    Human,
    /// Pretty-printed JSON doctor report.
    Json,
}

#[derive(Debug)]
struct CliError {
    exit_code: u8,
    message: String,
}

fn main() -> ExitCode {
    let arguments = std::env::args_os().collect::<Vec<_>>();
    let cli = match Cli::try_parse_from(arguments.iter().cloned()) {
        Ok(cli) => cli,
        Err(error) => {
            let exit_code = u8::try_from(error.exit_code()).unwrap_or(exit::INTERNAL);
            if print_sanitized_clap_error(&arguments, error.use_stderr()).is_err() {
                return ExitCode::from(exit::IO);
            }
            return ExitCode::from(exit_code);
        }
    };

    match run(cli) {
        Ok(exit_code) => ExitCode::from(exit_code),
        Err(error) => {
            let _ = writeln!(
                io::stderr().lock(),
                "error: {}",
                sanitize_terminal_text(&error.message)
            );
            ExitCode::from(error.exit_code)
        }
    }
}

fn print_sanitized_clap_error(arguments: &[OsString], use_stderr: bool) -> io::Result<()> {
    let sanitized = arguments
        .iter()
        .map(|argument| sanitize_os_argument(argument))
        .collect::<Vec<_>>();
    let rendered = match Cli::try_parse_from(sanitized) {
        Err(error) => error.render().to_string(),
        Ok(_) if use_stderr => {
            String::from("error: invalid command-line arguments\n\nUsage: steward <COMMAND>\n")
        }
        Ok(_) => format!("steward {}\n", env!("CARGO_PKG_VERSION")),
    };
    let rendered = sanitize_clap_rendering(&rendered);
    if use_stderr {
        io::stderr().lock().write_all(rendered.as_bytes())
    } else {
        io::stdout().lock().write_all(rendered.as_bytes())
    }
}

fn sanitize_os_argument(argument: &OsStr) -> OsString {
    argument.to_str().map_or_else(
        || OsString::from("<non-UTF-8 argument>"),
        |argument| OsString::from(sanitize_terminal_text(argument)),
    )
}

fn sanitize_clap_rendering(rendered: &str) -> String {
    let mut sanitized = String::with_capacity(rendered.len());
    for segment in rendered.split_inclusive('\n') {
        if let Some(line) = segment.strip_suffix('\n') {
            let line = line.strip_suffix('\r').unwrap_or(line);
            sanitized.push_str(&sanitize_terminal_text(line));
            sanitized.push('\n');
        } else {
            sanitized.push_str(&sanitize_terminal_text(segment));
        }
    }
    sanitized
}

fn run(cli: Cli) -> Result<u8, CliError> {
    match cli.command {
        Command::Version => {
            write_stdout(&format!("{}\n", version_line()))?;
            Ok(exit::SUCCESS)
        }
        Command::Doctor { format, config } => {
            let outcome = doctor(&DoctorOptions {
                config_path: config,
            });
            let rendered = match format {
                DoctorFormat::Human => render_doctor_human(&outcome.report),
                DoctorFormat::Json => {
                    let mut json =
                        serde_json::to_string_pretty(&outcome.report).map_err(|error| {
                            CliError {
                                exit_code: exit::INTERNAL,
                                message: format!("cannot serialize doctor report: {error}"),
                            }
                        })?;
                    json.push('\n');
                    json
                }
            };
            write_stdout(&rendered)?;
            Ok(outcome.exit_code)
        }
        Command::Config {
            command:
                ConfigCommand::Validate {
                    path,
                    allow_unknown,
                },
        } => {
            let validated = validate_config_path(
                &path,
                ValidationOptions {
                    allow_unknown_keys: allow_unknown,
                },
            )
            .map_err(|error| CliError {
                exit_code: if error.is_io() {
                    exit::IO
                } else {
                    exit::CONFIG_INVALID
                },
                message: error.to_string(),
            })?;

            let mut message = format!(
                "valid: {} (configuration version {})",
                sanitize_terminal_text(&path.to_string_lossy()),
                validated.config.version.as_str()
            );
            if !validated.ignored_keys.is_empty() {
                message.push_str("; ignored unknown key(s): ");
                message.push_str(&sanitize_terminal_text(&validated.ignored_keys.join(", ")));
            }
            message.push('\n');
            write_stdout(&message)?;
            Ok(exit::SUCCESS)
        }
    }
}

fn write_stdout(message: &str) -> Result<(), CliError> {
    io::stdout()
        .lock()
        .write_all(message.as_bytes())
        .map_err(|error| CliError {
            exit_code: exit::IO,
            message: format!("cannot write stdout: {error}"),
        })
}
