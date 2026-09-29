# Setup

One-time steps to get this repository's pipeline from "code is here" to
"can publish a signed release." Everything here is meant to be run by
whoever has admin access to `rejmann/pvm-releases-build` on GitHub — nothing
here runs in CI.

## 1. Generate the manifest signing key

The `manifest` job in `.github/workflows/build.yml` signs `manifest.json`
with an ed25519 key. Generate it locally — this repo's tooling never
generates or handles the private half for you automatically; you run this
yourself:

```sh
go run ./cmd/genkey | gh secret set MANIFEST_SIGNING_KEY --repo rejmann/pvm-releases-build
git add keys/manifest-signing.pub
git commit -m "Add manifest signing public key"
```

What this does:

- Writes the **public** key to `keys/manifest-signing.pub` (commit it — this
  is what eventually gets embedded in `pvm` itself, in the same style as
  `internal/composer/keys` in the `pvm` repo, so pvm can verify manifests
  without trusting the network).
- Prints the **private** key (base64) to stdout only, piped directly into
  `gh secret set` — it never touches disk. `gh` needs to already be
  authenticated with admin access to this repo (`gh auth status`).

Until `MANIFEST_SIGNING_KEY` is set, the `manifest` job in `build.yml` fails
with a clear error. That's expected — everything else in the pipeline
(verify, build, smoke-test, even publishing the release itself) works
without it. Only the final signing step is blocked.

**Rotating the key later** is expensive: every previously published
`manifest.json` was signed with the old key, and `pvm` will have the old
public key embedded until a new `pvm` release ships with the new one. Don't
run `genkey` again casually — it refuses to overwrite an existing
`keys/manifest-signing.pub` for exactly this reason.

## 2. Create the `release` environment

`build.yml`'s `publish` and `manifest` jobs run under a GitHub Environment
called `release`. This is what lets you gate real publishes behind a manual
approval click, separate from the (unrestricted) build + smoke-test jobs:

1. Repo Settings → Environments → New environment → name it `release`.
2. Add yourself (or whoever should approve releases) as a required reviewer.
3. No environment secrets needed here — `MANIFEST_SIGNING_KEY` from step 1
   is a repo-level secret, visible to the `release` environment by default.

Without this environment configured, the `publish`/`manifest` jobs still run
(GitHub creates environments implicitly on first reference) but with no
approval gate — anyone who can trigger `build.yml` with `publish=true` can
publish immediately. Setting up the required reviewer is what makes the gate
real.

## 3. First dry run

Before letting `check-releases.yml`'s daily cron drive real releases,
validate the pipeline by hand:

```sh
gh workflow run build.yml -f version=8.4.26 -f targets=linux-amd64-musl -f publish=false
gh run watch
```

`publish=false` runs verify → build → smoke-test only — nothing gets
published, no signing happens, nothing is irreversible. Once that's green,
try the full target matrix, then a `publish=true` run once you're satisfied.

## 4. Let the scheduler take over

`check-releases.yml` runs daily and dispatches `build.yml` (with
`publish=true`) for any PHP patch php.net has published that isn't a release
here yet. No further action needed once steps 1–2 are done — new releases
show up in the `release` environment's approval queue as php.net ships them.
