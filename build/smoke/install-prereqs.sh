#!/bin/sh
# Installs what the smoke test itself needs before it can even extract the
# tarball or run the binary:
#   - tar: rockylinux:8-minimal and opensuse/leap:15.6 don't ship it.
#   - ca-certificates: so the static PHP binary's OpenSSL finds a CA bundle
#     (CLAUDE.md section 7). Debian is missing this by default even though
#     curl works there via a different trust path, so this isn't optional.
set -eu

if command -v apt-get >/dev/null 2>&1; then
  apt-get update -qq && apt-get install -y -qq tar ca-certificates
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y -q tar ca-certificates
elif command -v microdnf >/dev/null 2>&1; then
  microdnf install -y tar ca-certificates
elif command -v pacman >/dev/null 2>&1; then
  pacman -Sy --noconfirm tar ca-certificates
elif command -v zypper >/dev/null 2>&1; then
  zypper --non-interactive install tar ca-certificates
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache tar ca-certificates
else
  echo "install-prereqs: no known package manager found, assuming tar and CA certs are already present" >&2
fi
