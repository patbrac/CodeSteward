# Repository verification scripts

Create a virtual environment and install the pinned CI-only validator dependencies first:

```text
python -m venv .venv
.venv/bin/python -m pip install --requirement scripts/ci/requirements.txt
```

On Windows PowerShell, use `.\.venv\Scripts\python.exe` in place of `.venv/bin/python` for both installation and checks.

Then run the public checks from the repository root with that environment's Python (`.venv/bin/python` on macOS/Linux or `.venv\Scripts\python.exe` on Windows):

```text
.venv/bin/python scripts/ci/check_schema_examples.py
.venv/bin/python scripts/ci/check_public_contracts.py
.venv/bin/python -m unittest discover -s scripts/ci -p "test_*.py"
```

The scripts resolve the repository root from their own location, behave consistently on Windows, macOS, and Linux, and return nonzero after printing actionable validation errors. Schema validation uses the dependencies pinned by the CI setup; public-contract validation uses the Python standard library plus the local Cargo toolchain.

Together they verify:

- every schema is valid Draft 2020-12 and is registered locally by `$id`;
- every positive schema fixture passes and every negative fixture fails;
- required public trust, release, platform, and schema artifacts exist;
- third-party workflow actions are pinned to immutable commits;
- every Cargo package has the project license, MSRV, and repository-contained paths; and
- workspace crate dependencies exactly match the documented one-way architecture; and
- the boundary regression sample proves an inverted report-to-engine edge is rejected.

Release, no-network, determinism, and packaging helpers live in the adjacent `ci` and `release` directories. See [the development guide](../docs/contributing/development.md) and [release process](../docs/release/process.md) for the complete command sequence.
