#!/bin/sh
# Scenario 2 — Missing tests and weak ownership.
#
# Changes only src/runtime/cache.ts, which lives in an area covered solely by
# the fallback CODEOWNERS rule and has no matching test. The PR description is
# empty.
#
# Expected CodeSteward outcome (CONTRACTS 7.2):
#   Status:    Needs contributor action
#   Ownership: Partial       (CS-OWN-002, fallback-only)
#   Tests:     Missing matching updates (CS-TST-001)
#   Burden:    Medium
#   Findings:  CS-OWN-002, CS-TST-001, CS-DSC-001  (score 60)
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

# Touch runtime source only; do not add or update any test.
cat >> src/runtime/cache.ts <<'EOF'

// Report the least-recently-used key without evicting it.
export function oldestKey<K, V>(cache: LruCache<K, V>): K | undefined {
  return (cache as unknown as { store: Map<K, V> }).store.keys().next().value;
}
EOF

git add -A
git commit -q -m "runtime: expose the least-recently-used cache key"

# Empty description forces evaluation and triggers CS-DSC-001.
"$CODESTEWARD_BIN" scan \
  --base main \
  --head HEAD \
  --repo-root "$TMP_DIR" \
  --description ""
