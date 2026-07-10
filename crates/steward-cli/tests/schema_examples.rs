use std::fs;
use std::path::{Path, PathBuf};

use serde_json::Value;

struct SchemaCase {
    schema: &'static str,
    valid_dir: &'static str,
    invalid_dir: &'static str,
}

const CASES: &[SchemaCase] = &[
    SchemaCase {
        schema: "schemas/config/v0/steward-config.schema.json",
        valid_dir: "schemas/config/v0/examples/valid",
        invalid_dir: "schemas/config/v0/examples/invalid",
    },
    SchemaCase {
        schema: "schemas/plugin/v0/plugin-manifest.schema.json",
        valid_dir: "schemas/plugin/v0/examples/valid",
        invalid_dir: "schemas/plugin/v0/examples/invalid",
    },
    SchemaCase {
        schema: "schemas/export/v0/export-envelope.schema.json",
        valid_dir: "schemas/export/v0/examples/valid",
        invalid_dir: "schemas/export/v0/examples/invalid",
    },
];

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../..")
}

fn read_json(path: &Path) -> Value {
    let bytes =
        fs::read(path).unwrap_or_else(|error| panic!("cannot read {}: {error}", path.display()));
    serde_json::from_slice(&bytes)
        .unwrap_or_else(|error| panic!("{} is not valid UTF-8 JSON: {error}", path.display()))
}

fn json_files(directory: &Path) -> Vec<PathBuf> {
    let mut files = fs::read_dir(directory)
        .unwrap_or_else(|error| panic!("cannot list {}: {error}", directory.display()))
        .map(|entry| entry.expect("directory entry must be readable").path())
        .filter(|path| {
            path.extension()
                .is_some_and(|extension| extension == "json")
        })
        .collect::<Vec<_>>();
    files.sort();
    assert!(
        !files.is_empty(),
        "{} must not be empty",
        directory.display()
    );
    files
}

#[test]
fn phase_1_schemas_are_valid_draft_2020_12_and_examples_match_expectations() {
    let root = repository_root();
    for case in CASES {
        let schema_path = root.join(case.schema);
        let schema = read_json(&schema_path);
        assert_eq!(
            schema.get("$schema").and_then(Value::as_str),
            Some("https://json-schema.org/draft/2020-12/schema"),
            "{} must declare Draft 2020-12",
            schema_path.display()
        );
        assert!(
            jsonschema::draft202012::meta::is_valid(&schema),
            "{} must validate against the Draft 2020-12 meta-schema",
            schema_path.display()
        );

        let validator = jsonschema::draft202012::options()
            .build(&schema)
            .unwrap_or_else(|error| panic!("cannot compile {}: {error}", schema_path.display()));

        for example_path in json_files(&root.join(case.valid_dir)) {
            let example = read_json(&example_path);
            let errors = validator
                .iter_errors(&example)
                .map(|error| error.to_string())
                .collect::<Vec<_>>();
            assert!(
                errors.is_empty(),
                "valid example {} failed: {}",
                example_path.display(),
                errors.join("; ")
            );
        }

        for example_path in json_files(&root.join(case.invalid_dir)) {
            let example = read_json(&example_path);
            assert!(
                !validator.is_valid(&example),
                "invalid example {} unexpectedly passed",
                example_path.display()
            );
        }
    }
}
