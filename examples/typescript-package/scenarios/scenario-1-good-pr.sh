#!/bin/sh
# Scenario 1 — Good PR.
#
# Changes src/parser/tokenize.ts together with its matching test file
# tests/parser/tokenize.test.ts and supplies a real PR description.
#
# Expected CodeSteward outcome (CONTRACTS 7.2):
#   Status:    Ready for maintainer review
#   Ownership: Complete
#   Tests:     Present (matching test changed)
#   Burden:    Low
#   Findings:  none
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

# Edit production source and update its matching test in the same PR.
cat >> src/parser/tokenize.ts <<'EOF'

// Recognize a leading unary minus as a subtraction against zero.
export function normalizeMinus(input: string): string {
  return input.replace(/^\s*-/, "0 -");
}
EOF

cat >> tests/parser/tokenize.test.ts <<'EOF'

describe("tokenize (extended)", () => {
  it("tokenizes a subtraction", () => {
    expect(tokenize("5 - 2")).toEqual([
      { kind: "number", value: "5" },
      { kind: "minus", value: "-" },
      { kind: "number", value: "2" },
    ]);
  });
});
EOF

git add -A
git commit -q -m "parser: normalize leading unary minus and cover it with tests"

DESCRIPTION="Add support for normalizing a leading unary minus in the tokenizer and extend the parser unit tests to cover the new subtraction handling behavior."

"$CODESTEWARD_BIN" scan \
  --base main \
  --head HEAD \
  --repo-root "$TMP_DIR" \
  --description "$DESCRIPTION"
