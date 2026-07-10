#!/usr/bin/env python3
"""Validate every public JSON Schema and its positive/negative examples."""

from __future__ import annotations

import json
import sys
import tomllib
from pathlib import Path
from typing import Any

from jsonschema import FormatChecker
from jsonschema.validators import validator_for
from referencing import Registry, Resource


ROOT = Path(__file__).resolve().parents[2]
SCHEMAS = ROOT / "schemas"


def load_document(path: Path) -> Any:
    if path.suffix == ".json":
        return json.loads(path.read_text(encoding="utf-8"))
    if path.suffix == ".toml":
        with path.open("rb") as handle:
            return tomllib.load(handle)
    raise ValueError(f"unsupported example format: {path.relative_to(ROOT)}")


def schema_directory(example: Path) -> Path:
    for parent in example.parents:
        if parent == SCHEMAS.parent:
            break
        if any(parent.glob("*.schema.json")):
            return parent
    raise ValueError(f"no sibling schema directory for {example.relative_to(ROOT)}")


def select_schema(example: Path, document: Any, schemas: dict[str, tuple[Path, dict[str, Any]]]) -> str:
    if isinstance(document, dict) and isinstance(document.get("$schema"), str):
        declared = document["$schema"]
        if declared in schemas:
            return declared

    directory = schema_directory(example)
    local = [(uri, path.stem.removesuffix(".schema")) for uri, (path, _) in schemas.items() if path.parent == directory]
    if len(local) == 1:
        return local[0][0]
    name = example.stem
    direct = [uri for uri, stem in local if name == stem or name.startswith(stem + "-")]
    if len(direct) == 1:
        return direct[0]

    first_token = name.split("-", 1)[0]
    prefix = [uri for uri, stem in local if stem.startswith(first_token + "-")]
    if len(prefix) == 1:
        return prefix[0]
    raise ValueError(
        f"cannot unambiguously select a schema for {example.relative_to(ROOT)}; "
        "name it after its schema or add a recognized $schema declaration"
    )


def main() -> int:
    schema_paths = sorted(SCHEMAS.rglob("*.schema.json"), key=lambda path: path.as_posix())
    if not schema_paths:
        print("no schemas found", file=sys.stderr)
        return 1

    schemas: dict[str, tuple[Path, dict[str, Any]]] = {}
    failures: list[str] = []
    for path in schema_paths:
        try:
            schema = json.loads(path.read_text(encoding="utf-8"))
            uri = schema["$id"]
            if not isinstance(uri, str) or not uri:
                raise ValueError("$id must be a non-empty string")
            if uri in schemas:
                raise ValueError(f"duplicate $id also used by {schemas[uri][0].relative_to(ROOT)}")
            cls = validator_for(schema)
            cls.check_schema(schema)
            schemas[uri] = (path, schema)
        except Exception as error:  # noqa: BLE001 - aggregate all schema diagnostics
            failures.append(f"{path.relative_to(ROOT)}: {error}")

    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1

    registry = Registry().with_resources(
        (uri, Resource.from_contents(schema)) for uri, (_, schema) in schemas.items()
    )
    examples = sorted(
        (
            path
            for path in SCHEMAS.rglob("*")
            if path.is_file()
            and ("valid" in path.parts or "invalid" in path.parts)
            and path.suffix in {".json", ".toml"}
        ),
        key=lambda path: path.as_posix(),
    )
    if not examples:
        print("no schema examples found", file=sys.stderr)
        return 1

    valid_count = 0
    invalid_count = 0
    for example in examples:
        try:
            document = load_document(example)
            uri = select_schema(example, document, schemas)
            _, schema = schemas[uri]
            validator = validator_for(schema)(schema, registry=registry, format_checker=FormatChecker())
            errors = sorted(validator.iter_errors(document), key=lambda error: list(error.absolute_path))
            expects_valid = "valid" in example.parts and "invalid" not in example.parts
            if expects_valid:
                valid_count += 1
                if errors:
                    paths = ["/" + "/".join(map(str, error.absolute_path)) for error in errors]
                    raise ValueError(f"expected valid; errors at {paths}")
            else:
                invalid_count += 1
                if not errors:
                    raise ValueError("expected rejection but validation succeeded")
        except Exception as error:  # noqa: BLE001 - aggregate all example diagnostics
            failures.append(f"{example.relative_to(ROOT)}: {error}")

    if valid_count == 0 or invalid_count == 0:
        failures.append("the corpus must contain at least one valid and one invalid example")
    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1

    print(f"validated {len(schema_paths)} schemas, {valid_count} valid examples, and {invalid_count} rejected examples")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
