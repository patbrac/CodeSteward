# Verify a steward development release

Replace VERSION and TARGET with the release values. The canonical repository identity is **patbrac/CodeSteward**.

## 1. Verify checksums

### Full release verification (recommended)

Download the complete release asset set into an empty directory. `gh release download` without a pattern downloads every uploaded asset, including `SHA256SUMS`, all three target archives, SBOMs, attestation bundles, and Linux ABI evidence:

~~~sh
mkdir "steward-$VERSION-release"
cd "steward-$VERSION-release"
gh release download "v$VERSION" --repo patbrac/CodeSteward --dir .
~~~

On Windows PowerShell:

~~~powershell
$ReleaseDirectory = Join-Path $PWD "steward-$env:VERSION-release"
New-Item -ItemType Directory -Path $ReleaseDirectory -ErrorAction Stop | Out-Null
gh release download "v$env:VERSION" --repo patbrac/CodeSteward --dir $ReleaseDirectory
Set-Location -LiteralPath $ReleaseDirectory
~~~

Then verify the release-wide manifest. This check intentionally fails if any file named by `SHA256SUMS` is missing.

On Linux:

~~~sh
sha256sum -c SHA256SUMS
~~~

On stock macOS:

~~~sh
shasum -a 256 -c SHA256SUMS
~~~

On Windows PowerShell:

~~~powershell
function Assert-Sha256List([string] $ListPath) {
    foreach ($Line in Get-Content -LiteralPath $ListPath) {
        if ($Line -notmatch '^([0-9a-fA-F]{64})  ([^\\/]+)$') {
            throw "Malformed checksum line in $ListPath"
        }
        $Expected = $Matches[1].ToLowerInvariant()
        $Name = $Matches[2]
        if (-not (Test-Path -LiteralPath $Name -PathType Leaf)) {
            throw "Missing release asset: $Name"
        }
        $Actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $Name).Hash.ToLowerInvariant()
        if ($Actual -ne $Expected) { throw "SHA-256 mismatch: $Name" }
    }
}
Assert-Sha256List "SHA256SUMS"
~~~

`SHA256SUMS` covers every archive, SBOM, attestation bundle, and Linux ABI-evidence file. Per-archive `.sha256` sidecars are separate convenience manifests and are not entries in the release-wide manifest.

### One-target sidecar verification

If you need only a checksum for one target, download exactly that target's archive and matching `.sha256` sidecar from the same release. To continue with target-specific attestation and SBOM verification, additionally download its `.spdx.json`, `.provenance.bundle.json`, and `.sbom.bundle.json` files. Do **not** run `SHA256SUMS` against this partial directory: it correctly requires the full release.

On Linux:

~~~sh
ASSET="steward-$VERSION-$TARGET.tar.gz"
sha256sum -c "$ASSET.sha256"
~~~

On stock macOS:

~~~sh
ASSET="steward-$VERSION-$TARGET.tar.gz"
shasum -a 256 -c "$ASSET.sha256"
~~~

On Windows PowerShell, define the same strict parser used above and verify only the sidecar:

~~~powershell
$Asset = "steward-$env:VERSION-$env:TARGET.zip"
Assert-Sha256List "$Asset.sha256"
~~~

The sidecar covers only its executable archive.

For the Linux target, also inspect the adjacent .glibc.txt evidence. It records the maximum referenced GLIBC symbol version and must remain at or below the qualified 2.39 baseline.

## 2. Verify the keyless signature and provenance online

Install a current GitHub CLI, authenticate if the repository is private, then run:

~~~sh
ASSET="steward-$VERSION-$TARGET.tar.gz"
gh attestation verify "$ASSET" \
  --repo patbrac/CodeSteward \
  --signer-workflow patbrac/CodeSteward/.github/workflows/release.yml
~~~

For Windows, use the .zip asset name. A successful default verification validates the Sigstore signature, archive digest, GitHub identity, source repository, and SLSA build-provenance predicate.

Verify the separately signed SPDX SBOM predicate:

~~~sh
gh attestation verify "$ASSET" \
  --repo patbrac/CodeSteward \
  --signer-workflow patbrac/CodeSteward/.github/workflows/release.yml \
  --predicate-type https://spdx.dev/Document/v2.3
~~~

The same online checks in Windows PowerShell are:

~~~powershell
$Asset = "steward-$env:VERSION-$env:TARGET.zip"
gh attestation verify $Asset --repo patbrac/CodeSteward --signer-workflow patbrac/CodeSteward/.github/workflows/release.yml
gh attestation verify $Asset --repo patbrac/CodeSteward --signer-workflow patbrac/CodeSteward/.github/workflows/release.yml --predicate-type https://spdx.dev/Document/v2.3
~~~

The GitHub keyless artifact attestation is the Phase 1 development signature. There is intentionally no detached GPG, Authenticode, or Apple-notarization signature yet. Verification proves which reviewed workflow and source commit produced the exact bytes; it does not make the binary trusted by the Windows or macOS application-signing UI.

## 3. Verify without a network connection

While online and before entering the isolated environment, obtain a fresh trusted root:

~~~sh
gh attestation trusted-root > trusted_root.jsonl
~~~

Windows PowerShell 5.1-safe UTF-8 export:

~~~powershell
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$RootLines = gh attestation trusted-root
[IO.File]::WriteAllText((Join-Path $PWD "trusted_root.jsonl"), (($RootLines -join [Environment]::NewLine) + [Environment]::NewLine), $Utf8NoBom)
~~~

Move the archive, matching provenance and SBOM bundle files, trusted_root.jsonl, and a compatible GitHub CLI into the offline environment. Then run:

~~~sh
gh attestation verify "$ASSET" \
  --repo patbrac/CodeSteward \
  --bundle "$ASSET.provenance.bundle.json" \
  --custom-trusted-root trusted_root.jsonl

gh attestation verify "$ASSET" \
  --repo patbrac/CodeSteward \
  --bundle "$ASSET.sbom.bundle.json" \
  --custom-trusted-root trusted_root.jsonl \
  --predicate-type https://spdx.dev/Document/v2.3
~~~

Windows PowerShell offline verification:

~~~powershell
$Asset = "steward-$env:VERSION-$env:TARGET.zip"
gh attestation verify $Asset --repo patbrac/CodeSteward --bundle "$Asset.provenance.bundle.json" --custom-trusted-root trusted_root.jsonl
gh attestation verify $Asset --repo patbrac/CodeSteward --bundle "$Asset.sbom.bundle.json" --custom-trusted-root trusted_root.jsonl --predicate-type https://spdx.dev/Document/v2.3
~~~

Refresh the trusted root whenever importing newly signed material. Offline verification cannot report key revocation that occurred after the trusted root was exported.

## 4. Inspect the SBOM

The SBOM is SPDX JSON and is also cryptographically bound to the archive by the SBOM attestation. At minimum, verify that its creation information names Syft and review its package/license inventory:

~~~sh
jq '.spdxVersion, .creationInfo.creators, [.packages[] | {name, versionInfo, licenseConcluded}]' \
  "$ASSET.spdx.json"
~~~

After extracting the archive, inspect **THIRD_PARTY_LICENSES.txt**. It is generated from the target-filtered, locked non-development Cargo graph and contains each compiled dependency's declared SPDX expression and bundled license/notice text. Archive verification fails if that material is missing or empty.

## 5. Install, run offline, and uninstall

Linux/macOS (user-local install; no administrator privileges):

~~~sh
tar -xzf "$ASSET"
mkdir -p "$HOME/.local/bin"
install -m 0755 "steward-$VERSION-$TARGET/steward" "$HOME/.local/bin/steward"
"$HOME/.local/bin/steward" version
"$HOME/.local/bin/steward" doctor
rm "$HOME/.local/bin/steward"
~~~

Windows PowerShell:

~~~powershell
Expand-Archive $Asset -DestinationPath .\steward-release
$Bin = Get-ChildItem .\steward-release -Recurse -Filter steward.exe | Select-Object -First 1
Get-AuthenticodeSignature -LiteralPath $Bin.FullName
& $Bin.FullName version
& $Bin.FullName doctor
Remove-Item -Recurse -LiteralPath .\steward-release
~~~

The CI clean-install test additionally blocks outbound network access at the native OS boundary while running the default surface, version, doctor, and config validation.

## Expected operating-system warnings

- **Windows:** Get-AuthenticodeSignature reports NotSigned, and Microsoft Defender SmartScreen or organizational application-control policy may warn or block the executable. This is expected for the Phase 1 zip. Do not disable SmartScreen, execution policy, antivirus, or organizational controls. Verify SHA256SUMS and both GitHub attestations first; then run only from a user-owned directory if local policy permits. Otherwise build the tagged source locally or wait for an Authenticode-signed package.
- **macOS:** the archive is not code-signed or notarized. Gatekeeper may reject it or require explicit approval. Do not remove quarantine attributes globally or disable Gatekeeper. Verify the checksums and both GitHub attestations first, inspect the exact binary with **spctl --assess --type execute --verbose=4 PATH**, and use the specific System Settings approval flow only if local policy permits. Otherwise build the tagged source locally or wait for a notarized package.
- **Linux:** the keyless GitHub attestation is the development signature; there is no distro package signature. Keep the install user-local and verify before execution.

Delete any unverified or blocked development artifact. Authenticode signing, Apple signing/notarization, and native installers remain explicit deferred gates rather than silent claims.

Primary GitHub verification guidance:

- https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations
- https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/verify-attestations-offline
- https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/verify-release-integrity
