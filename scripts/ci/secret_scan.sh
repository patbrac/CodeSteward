#!/usr/bin/env bash
set -euo pipefail

# Pinned binary plus an in-repository digest avoids a mutable container tag.
readonly VERSION="3.95.9"
readonly ARCHIVE="trufflehog_${VERSION}_linux_amd64.tar.gz"
readonly SHA256="f6d1106b85107d79527ed7a5b98b592beadd8b770dc3c9e8c1ad99e1b2cf127e"
readonly URL="https://github.com/trufflesecurity/trufflehog/releases/download/v${VERSION}/${ARCHIVE}"

temp_dir="$(mktemp -d)"
trap 'rm -rf -- "$temp_dir"' EXIT

curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  --output "$temp_dir/$ARCHIVE" "$URL"
printf '%s  %s\n' "$SHA256" "$temp_dir/$ARCHIVE" | sha256sum --check --strict
tar -xzf "$temp_dir/$ARCHIVE" -C "$temp_dir" trufflehog

"$temp_dir/trufflehog" git "file://$PWD" \
  --results=verified,unknown \
  --fail \
  --no-update \
  --github-actions
