# pvm-releases-build

Builds and publishes verifiable PHP binaries — Linux (musl, static) and
macOS — for [`pvm`](https://github.com/rejmann/pvm) to install without
depending on apt/dnf/pacman/zypper/brew. This repository plays the same role
for Linux/macOS that windows.php.net already plays for Windows: compile
once in CI, publish the binary, publish a signed index of what you get.

Design and rationale live in the `pvm` repo's `CLAUDE.md` (the proposal this
implements). Short version:

- **php.net publishes source, not Linux/macOS binaries.** Nobody else's
  prebuilt binaries come with a checksum you can automate against (see the
  CLAUDE.md's survey of static-php.dev, php-builder, etc.), so this repo
  builds its own from verified source.
- **Public repo, not because secrecy doesn't matter, but because it
  doesn't help here.** Free Actions minutes (including macOS and arm64),
  no-token downloads for `pvm install`, and a visible build workflow are
  the actual "verifiable origin" story.
- **Public ≠ trusted.** A checksum listed next to the file it describes only
  proves internal consistency. The `manifest.json` this repo publishes is
  signed with an ed25519 key whose private half never leaves a GitHub Actions
  secret — so even a compromised repo can't make `pvm` install something
  this key didn't sign. See `SETUP.md` and `keys/manifest-signing.pub`.

## What's here

| Path | Purpose |
|---|---|
| `.github/workflows/build.yml` | verify source → build (Phase 1 matrix) → smoke-test → publish → sign manifest |
| `.github/workflows/check-releases.yml` | daily: finds new php.net patches, dispatches `build.yml` |
| `cmd/releasecheck` | discovers PHP branches ≥ `-min-version` from php-src, diffs against published `php-*` releases here |
| `cmd/manifestgen` | assembles + signs `manifest.json` |
| `cmd/genkey` | one-time: generates the ed25519 signing key (see `SETUP.md`) |
| `internal/manifest` | shared Go types + sign/verify — the same code path CI and (eventually) `pvm` use |
| `build/extensions.txt` | the Phase 1 extension set, single source of truth |
| `build/craft/*.yml.tmpl` | `spc craft.yml` templates per target family |
| `build/smoke/` | the 7-distro smoke-test matrix (Dockerfile + scripts) |
| `scripts/verify-php-source.sh` | SHA-256 + GPG verification of the php.net source tarball |
| `scripts/php-release-managers.txt` | pinned PGP fingerprints trusted to sign php.net sources |
| `scripts/spc-checksums.txt` | pinned SHA-256 of the `static-php-cli` release binaries this repo uses |
| `manifest.schema.json` | the format of the signed manifest |
| `THIRD_PARTY_LICENSES.md` | PHP License 3.01 + LGPL relink notice for what's statically linked in |

## Phase 1 scope

linux-amd64 / linux-arm64 (musl static), darwin-amd64 / darwin-arm64. No
dynamic loading of extensions yet (`dl()` always returns `false` on a static
musl binary) — that's Phase 2. Windows isn't built here; `pvm` keeps using
the official `releases.json` from windows.php.net for that.

## Setup / operating this repo

See `SETUP.md` for the one-time steps (signing key, release approval gate)
and how to run a safe dry build before trusting the scheduler.

## License

Open source, non-profit, no formal license file — the tooling here (Go
code, scripts, workflows) is meant to be read, reused, and built upon by
anyone. The PHP binaries this repo publishes carry the PHP License 3.01 and
whatever LGPL obligations apply to statically linked libraries; see
`THIRD_PARTY_LICENSES.md` for that.
