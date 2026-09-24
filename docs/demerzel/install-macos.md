# macOS installation

**Status: not published.** The formula is tap-ready at
[`packaging/homebrew/demerzel.rb`](../../packaging/homebrew/demerzel.rb), but no
private tap repository or release assets have been created/published by this
work. The Homebrew command below becomes usable only after the publication gates
at the end of this guide are completed.

## Private Homebrew install

Authenticate once with an account that can read the private Demerzel repository
and tap:

```sh
gh auth login --hostname github.com --scopes repo
```

After the approved tap contains the versioned formula and its matching signed
GitHub Release exists, installation is one command:

```sh
HOMEBREW_GITHUB_API_TOKEN="$(gh auth token)" brew install a-mad-av8r/tap/demerzel
```

The token is supplied through the environment and is not printed, stored in the
formula, or written to the release. The formula requires authenticated `gh`
access to the private release, verifies `manifest.sigstore.json` against the
expected identity
`https://github.com/a-mad-av8r/demerzel/.github/workflows/release.yml@refs/tags/v2.*`,
then checks the selected macOS binary against the signed manifest. It does not
use the donor project's releases. No `curl | sh` bootstrap is used.

The installed binary is in the Homebrew prefix. The formula provisions explicit
age custody on first install and registers a launchd service definition; start
it only when ready:

```sh
brew services start demerzel
```

The service binds to loopback and uses:

- `DATA_DIR=$HOME/.demerzel` (mode `0700`),
- `$HOME/.config/demerzel/identity.txt` (mode `0600`, outside `DATA_DIR`),
- the recipient derived from that identity at service start.
- `$HOME/.config/demerzel/data-dir` pins the canonical absolute `DATA_DIR`
  across upgrades.
- owner-only `DATA_DIR/control.sock` for the secret-bearing account CLI.

The new Homebrew default does not discover or move an old source-development
`./data` directory. Fresh installs use `~/.demerzel`; old development data stays
untouched at its previous canonical path. Before changing roots, either keep
that exact absolute path explicitly on first formula install or complete the
supervised database-plus-custody restore/import runbook. `HOMEBREW_DEMERZEL_DATA_DIR`
pins an absolute path at first install; later attempts to change the pin fail
closed. An existing database without its matching age identity also fails
closed. Never copy or rotate age ciphertext by itself.

To deliberately retain an existing absolute `DATA_DIR` on first formula install,
pass it explicitly; the formula pins the path for later upgrades:

```sh
HOMEBREW_DEMERZEL_DATA_DIR="/absolute/old/data/path" \
  HOMEBREW_GITHUB_API_TOKEN="$(gh auth token)" brew install a-mad-av8r/tap/demerzel
```

The browser and data-plane HTTP listener remains on loopback port 3001. Account
CLI commands use only the owner's Unix socket, never a plain HTTP endpoint; run
the CLI as the same user with the same `DATA_DIR` as launchd:

```sh
DATA_DIR="$HOME/.demerzel" "$(brew --prefix)/bin/demerzel" accounts list
```

Do not move or replace `~/.demerzel`, change its canonical absolute `DATA_DIR`,
or generate a replacement identity against its database. Keychain/Secret
Service custody is not used for unattended macOS service mode until stable
Developer-ID signing and a real upgrade/ACL test are release gates. The age
identity is created from first boot and must be backed up separately from the
data directory.

Earlier development builds may have used a repository-relative `./data`
directory. Homebrew does not move that data or its path-bound Keychain item to
`~/.demerzel`. Before starting the service against old state, follow the
[key-custody restore/import procedure](key-custody.md) with the original master
key and a verified database backup; never regenerate age custody over an
unmigrated database.

For non-secret vault escrow/recovery metadata, record the read-only locator:

```sh
demerzel key-locator --data-dir "$HOME/.demerzel"
```

The returned identifier is not a key and does not prove a restore. A complete
recovery set includes the database (and SQLite sidecars when present),
`auth.key`, the external age identity plus `encryption.key.age`,
`runtime-state.checkpoint.json`, and operator configuration. Back up while the
service is stopped and verify a restore before relying on it.

Use the [credential accounts guide](accounts.md) for account operations and
the [key-custody runbook](key-custody.md) for restore procedures.

## Local formula install from generated artifacts

A generated release-asset directory contains the tap-ready formula, signed
manifest and bundle, `SHA256SUMS`, both macOS binaries, and executable
installer/smoke tools. Before Homebrew evaluates a generated formula, verify its
manifest signature and hashes:

```sh
release_dir="/path/to/generated/release-assets"
identity='^https://github\.com/a-mad-av8r/demerzel/\.github/workflows/release\.yml@refs/tags/v2\.[0-9]+\.[0-9]+(-[A-Za-z0-9][A-Za-z0-9.-]*)?$'
cosign verify-blob \
  --bundle "$release_dir/manifest.sigstore.json" \
  --certificate-identity-regexp "$identity" \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "$release_dir/manifest.json"
for tool in demerzel.rb install.sh verify-release.sh local-smoke.sh; do
  expected="$(jq -er --arg name "$tool" '.assets[] | select(.name == $name) | .sha256' "$release_dir/manifest.json")"
  actual="$(shasum -a 256 "$release_dir/$tool" | cut -d ' ' -f 1)"
  test "$actual" = "$expected"
done
(cd "$release_dir" && shasum -a 256 -c SHA256SUMS)
```

Install directly from these generated artifacts with one Homebrew command and
no Git checkout:

```sh
HOMEBREW_DEMERZEL_ARTIFACT_DIR="$release_dir" brew install --formula "$release_dir/demerzel.rb"
```

For a signed upgrade/rollback smoke, provide two generated artifact directories:

```sh
bash "$new_dir/local-smoke.sh" "$old_dir" "$new_dir"
```

The smoke needs `cosign`, `jq`, and `age-keygen`; it exercises real binaries and
does not need a Git checkout. It has not been run because no signed release
artifact directories exist.

## Upgrade, rollback, and uninstall

`brew upgrade demerzel` changes the Homebrew-managed binary; it never migrates or
removes `~/.demerzel` or the external age identity. Rollback to a prior signed
version uses that version's generated formula/artifact directory, for example:

```sh
HOMEBREW_DEMERZEL_ARTIFACT_DIR="/path/to/previous/release-assets" brew reinstall --formula "/path/to/previous/release-assets/demerzel.rb"
```

Rollback changes code only. It does not rewind database migrations or restore
secrets; keep a consistent state backup and verify compatibility before
activating an older binary. Keep the same absolute data root and key material.

Uninstalling Homebrew software preserves `~/.demerzel` and
`~/.config/demerzel/identity.txt`:

```sh
brew services stop demerzel
brew uninstall demerzel
```

The standalone generated-artifact installer also preserves data by default.
Its `uninstall --purge` option removes only the app-owned `~/.demerzel` directory
and requires typing the exact absolute path; the external identity remains
untouched.

## Publication prerequisites

Before the command at the top can be used, an authorized release owner must:

1. Create/approve the private `a-mad-av8r/homebrew-tap` repository and grant
   repository-read access; this worker does not create or push a tap.
2. Copy the versioned formula into the tap's `Formula/demerzel.rb` and set it to
   the exact approved stable release version.
3. Configure the `demerzel-release` GitHub Environment with required reviewers
   and set repository variable `DEMERZEL_RELEASE_APPROVED_TAG` to the exact
   authorized tag.
4. Approve a strict `v2.x` tag, complete release checks, and publish the signed
   manifest, bundle, checksums, and binary assets to the private GitHub Release.
5. Verify the release-identity certificate, formula's binary architecture and
   digest, first boot age custody, service restart, backup, and a real upgrade /
   rollback restore drill before treating launchd operation as supported.

No tap, package, GitHub Release, or production service is claimed by this guide.
