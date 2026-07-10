#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "reproducible_linux.sh must run on Linux" >&2
  exit 2
fi

readonly target="x86_64-unknown-linux-gnu"
readonly first="${RUNNER_TEMP:-/tmp}/steward-repro-a"
readonly second="${RUNNER_TEMP:-/tmp}/steward-repro-b"
readonly report="reproducibility-delta.txt"
rm -rf -- "$first" "$second"
rm -f -- "$report"

export CARGO_INCREMENTAL=0
export SOURCE_DATE_EPOCH="$(git log -1 --pretty=%ct)"
export RUSTFLAGS="-C debuginfo=0 -C strip=symbols -C link-arg=-Wl,--build-id=none"

CARGO_TARGET_DIR="$first" cargo build --locked --release -p steward-cli --target "$target"
CARGO_TARGET_DIR="$second" cargo build --locked --release -p steward-cli --target "$target"

binary_a="$first/$target/release/steward"
binary_b="$second/$target/release/steward"
sha256sum "$binary_a" "$binary_b"

if ! cmp --silent "$binary_a" "$binary_b"; then
  {
    echo "The two isolated Linux release builds were not byte-identical."
    echo "SOURCE_DATE_EPOCH=$SOURCE_DATE_EPOCH"
    sha256sum "$binary_a" "$binary_b"
    file "$binary_a" "$binary_b"
    readelf --notes "$binary_a" || true
    readelf --notes "$binary_b" || true
  } > "$report"
  cat "$report" >&2
  exit 1
fi

for command in version --help; do
  "$binary_a" "$command" > "$first/$command.stdout" 2> "$first/$command.stderr"
  "$binary_b" "$command" > "$second/$command.stdout" 2> "$second/$command.stderr"
  cmp "$first/$command.stdout" "$second/$command.stdout"
  cmp "$first/$command.stderr" "$second/$command.stderr"
done

echo "two isolated Linux builds are byte-for-byte identical"
