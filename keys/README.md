# keys/

This directory holds `manifest-signing.pub` once it's generated — see
`SETUP.md` step 1 (`go run ./cmd/genkey`). It's intentionally not committed
yet: `genkey` refuses to run if `manifest-signing.pub` already exists (to
avoid an accidental key rotation), so this repo ships without one until
whoever holds admin access generates it.

This is the public half of the ed25519 key `manifest.json` is signed with.
Once generated and committed here, it's also what eventually gets embedded
into `pvm` itself (mirroring `internal/composer/keys` in the `pvm` repo) so
`pvm` can verify a manifest without trusting the network for anything but
the bytes it's about to check.
