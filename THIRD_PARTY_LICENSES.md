# Third-party licenses

This repository builds and publishes PHP binaries. This file explains the
licensing of what's *inside* those binaries. The tooling that builds them
(this repo's own Go code, scripts, workflows) is open source and non-profit,
without a formal license file — see the note in README.md.

**This is not legal advice.** It's a good-faith explanation of the licensing
approach so a reviewer or a lawyer knows exactly what to check. Per the
`pvm` project's own CLAUDE.md (section 12), a legal review is recommended
before the first real public release under this pipeline, specifically
because of the LGPL point below.

## PHP itself: PHP License 3.01

Every binary published here is built from the unmodified PHP source
distributed by php.net (verified — see `scripts/verify-php-source.sh`). The
PHP License 3.01 permits redistributing binaries built from it, provided the
license text accompanies the distribution. A copy travels with every build
in `licenses/` inside `licenses-<target>.tar.gz` (see below), and the full
text is at <https://www.php.net/license/3_01.txt>.

## Statically linked libraries

Every extension in `build/extensions.txt` pulls in some set of C libraries,
all statically linked into the one `php` binary — that's the whole point of
the musl static build (see `~/Projects/pvm`'s CLAUDE.md sections 6-8: no
`.so` loading at runtime). Static linkage means each of those libraries'
licenses travels with the binary that embeds it, not just with this
repository.

Every build job runs `spc dump-license --for-extensions=<the list>` (static-
php-cli's own license-dumping command) and publishes the result as
`licenses-<target>.tar.gz` on the same GitHub Release as the binary it
describes — so the exact license set for the exact binary someone downloaded
is one file away, not a separate manually-maintained document that can drift.

**The LGPL libraries in the default set** — GMP (`gmp`) and GNU gettext
(`gettext`) — are where static linking matters legally. The LGPL's core
condition for static linking is that the *means to relink* the work against
a different version of the library must be available. This repository
satisfies that by construction, not as an afterthought:

- The exact PHP source, extension list, and `craft.yml` recipe that produced
  a given binary are public and versioned in this repo (`build/extensions.txt`,
  `build/craft/*.yml.tmpl`, the workflow run that built it).
- `static-php-cli` itself (MIT) is the public, pinned toolchain that does the
  linking — `scripts/spc-checksums.txt` pins the exact release used.
- Anyone can re-run the same build with a different version of GMP/gettext
  and get a working relinked binary, using nothing but public tools and this
  repo's own recipe.

Don't take this as the exhaustive list of what's actually linked in, though
— `spc` pulls in further transitive dependencies (e.g. curl was built
*without* idn2 support, confirmed from a real build log, but that kind of
detail changes as the extension set or spc version changes). The
`licenses-<target>.tar.gz` for each release (next section) is the exact,
generated-at-build-time answer for that specific binary; this paragraph is
just the two libraries worth calling out explicitly.

If a stricter reading is wanted (e.g. shipping the LGPL libraries' own
source alongside each release, not just linking to where `spc` fetches them
from), that's a deliberate follow-up — flag it in a PR rather than assuming
the current approach is the last word.

## Where to look per release

Each GitHub Release (`php-<version>`) includes, per target:

- `php-<version>-<target>.tar.gz` — the binary.
- `licenses-<target>.tar.gz` — every library license that binary embeds,
  dumped straight from `spc dump-license` at build time.

The signed `manifest.json` (on the `manifest` release) lists the exact
`extensions` compiled into each artifact, which is what `spc dump-license`
was run against.
