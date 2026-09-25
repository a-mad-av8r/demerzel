# macOS installation

Demerzel's source repository is public. **There is not yet a signed GitHub release or a Homebrew tap**, so neither packaged installation command below is available today. Build the public source revision until an authorised release and tap are published.

## Install from a signed release with curl

This path downloads the public release assets, verifies the Sigstore identity and installer hashes, then runs the already-verified native installer. It does not use `gh`, a GitHub token or `curl | sh`.

Requirements: macOS on Intel or Apple silicon, `curl`, `jq`, `cosign` and `age-keygen`. Homebrew can provide the verification and custody tools:

```sh
brew install cosign jq age
```

After the first approved public release, download and verify it as follows:

```bash
set -euo pipefail
umask 077

release_dir="$(mktemp -d)"
chmod 700 "$release_dir"
tag="$(curl --fail --location --silent --show-error \
  https://api.github.com/repos/a-mad-av8r/demerzel/releases/latest | jq -er '.tag_name')"
if [[ ! "$tag" =~ ^v2\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]]; then
  printf 'Unexpected Demerzel release tag: %s\n' "$tag" >&2
  exit 1
fi
case "$(uname -m)" in
  arm64) asset=demerzel-macos-arm64 ;;
  x86_64) asset=demerzel-macos-amd64 ;;
  *) printf 'Unsupported macOS architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac
release_base="https://github.com/a-mad-av8r/demerzel/releases/download/${tag}"
for name in "$asset" manifest.json manifest.sigstore.json install.sh verify-release.sh; do
  curl --fail --location --silent --show-error --proto '=https' --tlsv1.2 \
    "$release_base/$name" --output "$release_dir/$name"
done

escaped_tag="${tag//./\\.}"
identity="^https://github\\.com/a-mad-av8r/demerzel/\\.github/workflows/release\\.yml@refs/tags/${escaped_tag}$"
cosign verify-blob \
  --bundle "$release_dir/manifest.sigstore.json" \
  --certificate-identity-regexp "$identity" \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "$release_dir/manifest.json"
test "$(jq -er '.tag' "$release_dir/manifest.json")" = "$tag"
for tool in "$asset" install.sh verify-release.sh; do
  expected="$(jq -er --arg name "$tool" '.assets[] | select(.name == $name) | .sha256' "$release_dir/manifest.json")"
  actual="$(shasum -a 256 "$release_dir/$tool" | cut -d ' ' -f 1)"
  test "$actual" = "$expected"
done

bash "$release_dir/install.sh" install --artifacts "$release_dir"
```

The signed manifest is the source of truth for the downloaded binary and both executable installer tools. Verify it before running either script. The installer checks the binary again, installs under `~/.local/opt/demerzel`, creates an external age identity on an empty data directory and preserves runtime data on uninstall.

Run the installed gateway in a terminal:

```sh
"$HOME/.local/opt/demerzel/bin/demerzel"
```

The browser and data-plane listener binds to loopback on port 3001. The default standalone data directory is `~/.demerzel`; the external age identity is `~/.config/demerzel/identity.txt`. Keep the identity outside the data directory and back it up separately.

## Homebrew

The formula template is [`packaging/homebrew/demerzel.rb`](../../packaging/homebrew/demerzel.rb). It downloads public, versioned release assets with `curl`, verifies the Sigstore bundle against the Demerzel release workflow, checks the selected binary digest and provisions age custody. It does not require a GitHub token.

A tap repository and signed release do not exist yet. Once the public `a-mad-av8r/homebrew-tap` repository contains the versioned formula and the matching release has passed its approval gates, install and start the service with:

```sh
brew install a-mad-av8r/tap/demerzel
brew services start demerzel
```

The Homebrew service binds to loopback. It pins its absolute data path in `$HOME/.config/demerzel/data-dir` and stores the external identity in `$HOME/.config/demerzel/identity.txt`. Fresh installations use `~/.demerzel`. To keep an existing absolute data path on the first install only, set `HOMEBREW_DEMERZEL_DATA_DIR`:

```sh
HOMEBREW_DEMERZEL_DATA_DIR="/absolute/data/path" brew install a-mad-av8r/tap/demerzel
```

Homebrew refuses later attempts to change the pinned path. An existing database without its matching age identity also fails closed. Do not copy or rotate age ciphertext by itself.

## Runtime paths and account operations

For a Homebrew service:

- `DATA_DIR` is read from `$HOME/.config/demerzel/data-dir`; a fresh install defaults to `$HOME/.demerzel`.
- The age identity is `$HOME/.config/demerzel/identity.txt`, mode `0600`, outside `DATA_DIR`.
- The identity's recipient is supplied to the service at startup.
- Secret-bearing account commands use the owner-only `DATA_DIR/control.sock`, not the HTTP listener.

Run the account CLI as the same user and with the same `DATA_DIR` as the gateway:

```sh
demerzel accounts list
```

An old development `./data` directory is not discovered or moved automatically. Keep its original canonical absolute path and matching custody identity, or follow the supervised database-plus-custody restore procedure before moving state. An empty directory is not a credential migration.

For non-secret recovery metadata, record the read-only locator:

```sh
demerzel key-locator --data-dir "$(cat "$HOME/.config/demerzel/data-dir")"
```

The locator is not key material and does not prove a restore. A recovery set includes the database and any SQLite sidecars, `auth.key`, the matching external age identity and `encryption.key.age`, `runtime-state.checkpoint.json`, and operator configuration. Back up while the service is stopped and verify a restore before relying on it. See the [key-custody runbook](key-custody.md) and [credential accounts guide](accounts.md).

## Upgrade, rollback and uninstall

`brew upgrade demerzel` replaces only the Homebrew-managed binary. It does not migrate the pinned data directory or remove the external identity. Rollback changes code only; it does not reverse database migrations or restore secrets. Keep the same absolute data path and verify a consistent database-plus-key backup before activating an older version.

Stop and remove the Homebrew service without deleting runtime data:

```sh
brew services stop demerzel
brew uninstall demerzel
```

The standalone installer also preserves runtime data by default. To remove its binaries:

```sh
bash "$HOME/.local/opt/demerzel/install.sh" uninstall
```

The explicit standalone `uninstall --purge` operation removes only the app-owned `.demerzel` directory after requiring the exact absolute path. It leaves the external age identity untouched. Do not use purge as a substitute for a verified backup.

## Publication requirements

The release workflow accepts stable `v2.x.y` tags and prereleases that pass its version validation, but not build metadata. A release tag must point to a commit on `main` and pass the protected release approval. No signed GitHub release or Homebrew tap is published yet; treat the package commands above as unavailable until those external prerequisites exist. The planned version scheme is being tracked separately from the initial source publication.
