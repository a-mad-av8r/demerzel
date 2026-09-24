# Linux installation

**Status: not published.** Linux amd64 and arm64 binaries, the installer, signed
manifest, Sigstore bundle, and `SHA256SUMS` are release assets only after an
approved private GitHub Release. The installer accepts generated artifact
folders, so install, upgrade, rollback, and uninstall do not require a Git
checkout.

## Prerequisites

- Linux amd64 or arm64; native installer support is macOS/Linux only.
- Read access to private `a-mad-av8r/demerzel` through GitHub CLI (`gh`).
- `cosign`, `jq`, and `age-keygen` available locally.
- An owner-only, separately backed-up age identity. Do not place it in the data
  directory.

Authenticate explicitly before downloading; GitHub CLI keeps credentials in
its credential store. Do not enable shell tracing or print the token:

```sh
gh auth login --hostname github.com --scopes repo
```

## Install from signed release artifacts

Choose an approved version tag and download the release files into a private
staging directory:

```sh
release_dir="$(mktemp -d)"
chmod 700 "$release_dir"
tag=v2.0.0
GH_TOKEN="$(gh auth token)" gh release download "$tag" \
  --repo a-mad-av8r/demerzel \
  --pattern 'demerzel-linux-amd64' \
  --pattern 'demerzel-linux-arm64' \
  --pattern 'manifest.json' \
  --pattern 'manifest.sigstore.json' \
  --pattern 'SHA256SUMS' \
  --pattern 'install.sh' \
  --pattern 'verify-release.sh' \
  --pattern 'local-smoke.sh' \
  --dir "$release_dir"
```

The token is an environment value to `gh`; it is not printed or embedded in an
asset. The installer verifies the signature with Cosign against the expected
Demerzel GitHub Actions release-workflow identity and validates the selected
binary digest from the signed manifest before installing:

Verify the signed manifest and the hashes of the executable installer tools
before running either downloaded script:

```sh
identity='^https://github\.com/a-mad-av8r/demerzel/\.github/workflows/release\.yml@refs/tags/v2\.[0-9]+\.[0-9]+(-[A-Za-z0-9][A-Za-z0-9.-]*)?$'
cosign verify-blob \
  --bundle "$release_dir/manifest.sigstore.json" \
  --certificate-identity-regexp "$identity" \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "$release_dir/manifest.json"
for tool in install.sh verify-release.sh local-smoke.sh; do
  expected="$(jq -er --arg name "$tool" '.assets[] | select(.name == $name) | .sha256' "$release_dir/manifest.json")"
  actual="$(sha256sum "$release_dir/$tool" | cut -d ' ' -f 1)"
  test "$actual" = "$expected"
done
(cd "$release_dir" && sha256sum --check SHA256SUMS)
```


```sh
bash "$release_dir/install.sh" install --artifacts "$release_dir"
"$HOME/.local/opt/demerzel/bin/demerzel" help
```

The native installer maps Linux data to
`${XDG_DATA_HOME:-$HOME/.local/share}/demerzel` and stores the external age
identity at `$HOME/.config/demerzel/identity.txt`. It generates an age identity
only for an empty data directory. If existing data has lost its identity, the
installer fails closed; restore the original identity instead of creating a
replacement. The generated launcher uses the same absolute `DATA_DIR` on every
run.

The gateway creates the owner-only account-control socket at
`$DATA_DIR/control.sock`. Run secret-bearing account CLI commands as the same
Unix user with the same absolute `DATA_DIR`; the CLI uses this socket only and
does not accept a plain HTTP control URL. The browser/data-plane HTTP listener
remains on loopback port 3001.

```sh
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/demerzel" \
  "$HOME/.local/opt/demerzel/bin/demerzel" accounts list
```

### Optional systemd user service

The installed launcher supplies the same age identity and recipient on every
start. A user service can use the stable XDG data root:

```sh
mkdir -p "$HOME/.config/systemd/user"
cat >"$HOME/.config/systemd/user/demerzel.service" <<'EOF'
[Unit]
Description=Demerzel local gateway
After=network.target

[Service]
Type=simple
Environment=DATA_DIR=%h/.local/share/demerzel
WorkingDirectory=%h/.local/opt/demerzel
ExecStart=%h/.local/opt/demerzel/bin/demerzel
Restart=on-failure
UMask=0077
NoNewPrivileges=true

[Install]
WantedBy=default.target
EOF
systemctl --user daemon-reload
systemctl --user enable --now demerzel.service
```

If `XDG_DATA_HOME` is customized, set `Environment=DATA_DIR=...` to that exact
absolute path and use the same path for CLI commands, backups, upgrades, and
rollbacks. Do not relocate a populated data root.

Account CLI usage is documented in [accounts](accounts.md); the
[key-custody runbook](key-custody.md) defines the full recovery procedure.

## Upgrade and rollback from artifacts

Download the next and previous signed artifact sets into separate directories.
Upgrade atomically switches the `current` link after verifying the manifest;
previous version directories remain available for rollback:

```sh
bash "$new_dir/install.sh" install --artifacts "$new_dir"
bash "$new_dir/install.sh" rollback
```

`bash "$new_dir/local-smoke.sh" "$old_dir" "$new_dir"` exercises two signed
generated release directories: install, upgrade, run `help`, roll back, and
uninstall while checking that a sentinel in persistent data survives. It needs
`cosign`, `jq`, and `age-keygen`, but no source checkout. The smoke cannot be
run until two signed release artifact directories exist.

A rollback switches binaries only; it does not reverse database changes. Back
up and restore the database, `auth.key`, age identity plus `encryption.key.age`,
`runtime-state.checkpoint.json`, and operator configuration together. Preserve
the same absolute data path throughout; the canonical path is part of custody
and installation identity.

## Uninstall and purge

If you installed the sample systemd user service, stop it and remove only its
unit file before deleting binaries:

```sh
systemctl --user disable --now demerzel.service
rm -f "$HOME/.config/systemd/user/demerzel.service"
```

This does not touch the data or age identity.
Default uninstall removes the binary prefix and preserves all runtime data and
the external age identity:

```sh
bash "$release_dir/install.sh" uninstall
```


Data removal is intentionally separate and gated:

```sh
bash "$release_dir/install.sh" uninstall --purge
```

The script accepts only a data directory named `.demerzel` or `demerzel`, then
requires typing `PURGE` followed by the exact absolute path. It removes only
that data directory; it never removes the external identity. Do not use this to
replace a backup or erase state whose recovery has not been verified.

## Platform boundary

macOS and Linux are the supported native installer targets. Windows does not
support native vault or age custody in this work; Windows operators must supply
`ENCRYPTION_KEY` explicitly in a controlled environment. The Windows setup
installer is a pre-existing path and is not being productized here. No
in-application updater or donor release checker is included.
