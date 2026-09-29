#!/bin/sh
# Installs ca-certificates using whichever package manager the base image
# has, so the static PHP binary's OpenSSL finds /etc/ssl/cert.pem
# (CLAUDE.md section 7). Debian is missing this file by default even though
# curl works there via a different trust path, so this step isn't optional.
set -eu

if command -v apt-get >/dev/null 2>&1; then
  apt-get update -qq && apt-get install -y -qq ca-certificates
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y -q ca-certificates
elif command -v microdnf >/dev/null 2>&1; then
  microdnf install -y ca-certificates
elif command -v pacman >/dev/null 2>&1; then
  pacman -Sy --noconfirm ca-certificates
elif command -v zypper >/dev/null 2>&1; then
  zypper --non-interactive install ca-certificates
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache ca-certificates
else
  echo "install-ca-certs: no known package manager found, assuming CA certs are already present" >&2
fi
