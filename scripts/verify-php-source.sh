#!/usr/bin/env bash
# Downloads a php.net source tarball, verifies its SHA-256 against
# php.net's own releases JSON, verifies its detached GPG signature against a
# pinned release-manager fingerprint (scripts/php-release-managers.txt), and
# leaves the verified tarball at <out-dir>/php-<version>.tar.gz.
#
# This is the only place in the pipeline allowed to trust a php.net
# download unverified — build.yml calls this before anything else, and every
# later step operates on the tarball this script already checked.
#
# Usage: verify-php-source.sh <X.Y.Z> [out-dir]
#
# Limitation: php.net's releases JSON (releases/index.php?json&version=<X.Y>)
# only reports the *current latest* patch of a branch, not history. This
# script can only verify a version that php.net still reports as the latest
# of its branch. That's fine for the automated pipeline (it only ever builds
# the latest patch), but a manual workflow_dispatch for an older/EOL patch
# will fail here with a clear error instead of silently trusting an
# unverified download.
set -euo pipefail

version="${1:?usage: verify-php-source.sh <X.Y.Z> [out-dir]}"
out_dir="${2:-.}"
branch="${version%.*}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fingerprints_file="${script_dir}/php-release-managers.txt"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "verify-php-source: '$version' is not an X.Y.Z version" >&2
  exit 1
fi

mkdir -p "$out_dir"
work_dir="$(mktemp -d)"
gnupg_home="$(mktemp -d)"
trap 'rm -rf "$work_dir" "$gnupg_home"' EXIT
export GNUPGHOME="$gnupg_home"
chmod 700 "$gnupg_home"

echo "verify-php-source: checking php.net's releases feed for branch $branch..." >&2
feed_json="$(curl -fsSL "https://www.php.net/releases/index.php?json&version=${branch}")"
feed_version="$(echo "$feed_json" | jq -r '.version')"
if [[ "$feed_version" != "$version" ]]; then
  echo "verify-php-source: php.net reports $feed_version as the latest $branch patch, not $version." >&2
  echo "  This script only verifies the current latest patch of a branch (see script header)." >&2
  exit 1
fi
expected_sha256="$(echo "$feed_json" | jq -r '.source[] | select(.filename == "php-'"$version"'.tar.gz") | .sha256')"
if [[ -z "$expected_sha256" || "$expected_sha256" == "null" ]]; then
  echo "verify-php-source: no php-${version}.tar.gz entry in php.net's feed for branch $branch" >&2
  exit 1
fi

tarball="${work_dir}/php-${version}.tar.gz"
sig="${tarball}.asc"
echo "verify-php-source: downloading php-${version}.tar.gz and its signature..." >&2
curl -fsSL -o "$tarball" "https://www.php.net/distributions/php-${version}.tar.gz"
curl -fsSL -o "$sig" "https://www.php.net/distributions/php-${version}.tar.gz.asc"

actual_sha256="$(sha256sum "$tarball" | cut -d' ' -f1)"
if [[ "$actual_sha256" != "$expected_sha256" ]]; then
  echo "verify-php-source: SHA-256 mismatch for php-${version}.tar.gz" >&2
  echo "  expected: $expected_sha256" >&2
  echo "  actual:   $actual_sha256" >&2
  exit 1
fi
echo "verify-php-source: SHA-256 matches php.net's feed ($actual_sha256)" >&2

echo "verify-php-source: importing php.net's release-manager keyring..." >&2
curl -fsSL "https://www.php.net/distributions/php-keyring.gpg" | gpg --batch --dearmor >"${work_dir}/keyring.bin"
gpg --batch --import "${work_dir}/keyring.bin" >/dev/null 2>&1

echo "verify-php-source: verifying GPG signature..." >&2
status="$(gpg --batch --status-fd 1 --verify "$sig" "$tarball" 2>/dev/null || true)"
if ! echo "$status" | grep -q '^\[GNUPG:\] GOODSIG'; then
  echo "verify-php-source: GPG signature is not a GOODSIG:" >&2
  echo "$status" >&2
  exit 1
fi
signer_fpr="$(echo "$status" | awk '/^\[GNUPG:\] VALIDSIG/ {print $3}')"
if [[ -z "$signer_fpr" ]]; then
  echo "verify-php-source: could not extract signer fingerprint from GPG output" >&2
  exit 1
fi
if ! grep -qi "^${signer_fpr} " "$fingerprints_file"; then
  echo "verify-php-source: signature is a GOODSIG but from an UNPINNED key: $signer_fpr" >&2
  echo "  This may be a legitimate new php.net release manager — if so, add their" >&2
  echo "  fingerprint to $fingerprints_file via a reviewed PR. Refusing to trust it automatically." >&2
  exit 1
fi
signer_name="$(grep -i "^${signer_fpr} " "$fingerprints_file" | awk '{$1=""; print $0}' | sed 's/^ *//')"
echo "verify-php-source: signature verified, signed by pinned key: $signer_name ($signer_fpr)" >&2

dest="${out_dir}/php-${version}.tar.gz"
mv "$tarball" "$dest"
echo "verify-php-source: verified tarball at $dest" >&2

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  {
    echo "tarball_path=${dest}"
    echo "sha256=${actual_sha256}"
  } >>"$GITHUB_OUTPUT"
fi
echo "$actual_sha256"
