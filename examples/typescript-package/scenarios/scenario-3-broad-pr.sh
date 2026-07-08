#!/bin/sh
# Scenario 3 — Broad PR.
#
# Touches parser source, unowned runtime source, docs, the package manifest,
# and a CI release workflow all at once, with an empty description.
#
# Changed files:
#   src/parser/parse.ts
#   src/runtime/cache.ts
#   docs/usage.md
#   package.json
#   .github/workflows/release.yml
#
# Expected CodeSteward outcome (CONTRACTS 7.2):
#   Status:    High review burden
#   Burden:    High
#   Findings:  CS-OWN-002, CS-TST-001, CS-SCP-003, CS-SCP-004,
#              CS-DSC-001, CS-SNS-002, CS-SNS-003            (score 15)
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
EXAMPLE_DIR=$(dirname -- "$SCRIPT_DIR")
CODESTEWARD_BIN="${CODESTEWARD_BIN:-codesteward}"
# Resolve a relative binary path now, while we are still in the invocation
# directory; the scan below runs after we cd into the throwaway repo, so a path
# like ../../bin/codesteward would otherwise re-resolve against the temp dir and
# fail with exit 127. Bare command names (no slash) are left for PATH lookup.
case "$CODESTEWARD_BIN" in
  */*) CODESTEWARD_BIN=$(CDPATH= cd -- "$(dirname -- "$CODESTEWARD_BIN")" && pwd)/$(basename -- "$CODESTEWARD_BIN") ;;
esac

TMP_DIR=$(mktemp -d)
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

# Copy the example into an isolated repo.
cp -R "$EXAMPLE_DIR/." "$TMP_DIR/"
cd "$TMP_DIR"

git init -q -b main
git config user.email t@t
git config user.name t
git add -A
git commit -q -m "initial example package"

git checkout -q -b change

# 1. Parser source change.
cat >> src/parser/parse.ts <<'EOF'

// Convenience wrapper that parses and rounds to an integer result.
export function parseInteger(input: string): number {
  return Math.trunc(parse(input));
}
EOF

# 2. Unowned runtime source change (fallback ownership, no test).
cat >> src/runtime/cache.ts <<'EOF'

// Remove every entry from the cache.
export function clearCache<K, V>(cache: LruCache<K, V>): void {
  (cache as unknown as { store: Map<K, V> }).store.clear();
}
EOF

# 3. Docs change.
cat >> docs/usage.md <<'EOF'

## Parsing integers

```ts
import { parseInteger } from "example-typescript-package";

parseInteger("7 / 2"); // => 3
```
EOF

# 4. Package manifest change (sensitive: manifest).
cat > package.json <<'EOF'
{
  "name": "example-typescript-package",
  "version": "0.2.0",
  "private": true,
  "description": "Tiny TypeScript library used to demonstrate CodeSteward.",
  "license": "Apache-2.0",
  "main": "src/public/index.ts",
  "types": "src/public/index.ts",
  "scripts": {
    "test": "jest"
  }
}
EOF

# 5. CI release workflow change (sensitive: CI/release workflow).
mkdir -p .github/workflows
cat > .github/workflows/release.yml <<'EOF'
name: Release

on:
  push:
    tags:
      - "v*"

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: npm publish
EOF

git add -A
git commit -q -m "release plumbing plus parser, runtime, and docs updates"

# Empty description forces evaluation and triggers CS-DSC-001.
"$CODESTEWARD_BIN" scan \
  --base main \
  --head HEAD \
  --repo-root "$TMP_DIR" \
  --description ""
