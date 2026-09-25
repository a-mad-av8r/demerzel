# Linux installation

Demerzel's source repository is public. **There is not yet a signed GitHub release**, so the pre-built download below is unavailable today. Build the public source revision until an authorised release is published.

## Install from a signed release with curl

This path downloads the public release assets, verifies the Sigstore identity and installer hashes, then runs the already-verified native installer. It does not use `gh`, a GitHub token or `curl | sh`.

Requirements: Linux amd64 or arm64, `curl`, `jq`, `cosign` and `age-keygen`. Keep the age identity owner-only and outside the data directory; back it up separately.

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
  x86_64) asset=demerzel-linux-amd64 ;;
  aarch64|arm64) asset=demerzel-linux-arm64 ;;
  *) printf 'Unsupported Linux architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
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
  actual="$(sha256sum "$release_dir/$tool" | cut -d ' ' -f 1)"
  test "$actual" = "$expected"
done

bash "$release_dir/install.sh" install --artifacts "$release_dir"
"$HOME/.local/opt/demerzel/bin/demerzel"
```

This starts the gateway in the foreground. Leave it running and use another
terminal for account commands, or configure the optional systemd user service.

The signed manifest is the source of truth for the downloaded binary and both executable installer tools. Verify it before running either script. The installer checks the binary again, installs under `~/.local/opt/demerzel`, creates an external age identity only for an empty data directory and fails closed if existing data has lost its identity.

## Persistent data and account operations

On Linux, the standalone installer uses:

- `${XDG_DATA_HOME:-$HOME/.local/share}/demerzel` for persistent data;
- `$HOME/.config/demerzel/identity.txt` (mode `0600`) for the external age identity;
- an owner-only `DATA_DIR/control.sock` for secret-bearing account commands.

The generated launcher uses the same absolute `DATA_DIR` and age identity on every run. Run account commands as the same Unix user and with the same data directory as the gateway; the account CLI does not accept a plain HTTP control URL. The browser and data-plane listener remains on loopback port 3001.

Previous development installations may use `./data`; the installer does not move them to the XDG default. Keep the old canonical absolute path with the original custody identity, or follow the supervised database-plus-key restore procedure in the [key-custody runbook](key-custody.md). Do not start a new key against old ciphertext or assume an empty XDG directory contains the old accounts.

### Optional systemd user service

The following unit uses the installer's default XDG data path. If `XDG_DATA_HOME` is customised, set `DATA_DIR` to that same absolute path for both the service and account CLI.

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

Use the account CLI through the owner-only socket:

```sh
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/demerzel" \
  "$HOME/.local/opt/demerzel/bin/demerzel" accounts list
```

## Upgrade and rollback

Download each approved signed artifact set to a separate mode-`0700` directory. Do not execute a script from a new directory to verify itself. The first verified install keeps `install.sh` and `verify-release.sh` under `$HOME/.local/opt/demerzel`; use those trusted copies for upgrades:

```sh
trusted="$HOME/.local/opt/demerzel"
bash "$trusted/install.sh" install --artifacts "$new_dir"
bash "$trusted/install.sh" rollback
```

The installer uses its adjacent trusted verifier to authenticate the new binary and both installer tools before copying or executing them. The canonical `DATA_DIR` pin under `$HOME/.config/demerzel/data-dir` survives upgrades and uninstalls; requesting a different root fails closed. If the age identity directory was customised, pass the same `--config-dir` on upgrade.

Rollback changes binaries only; it does not reverse database migrations. Keep the database, `auth.key`, external age identity and `encryption.key.age`, `runtime-state.checkpoint.json`, and operator configuration as one restore-proven set. Preserve the same absolute data path throughout.

## Uninstall and purge

If the optional systemd user service is installed, stop it and remove only its unit file:

```sh
systemctl --user disable --now demerzel.service
rm -f "$HOME/.config/systemd/user/demerzel.service"
```

Default uninstall removes the binary prefix and preserves runtime data, the data-root pin and the external age identity:

```sh
bash "$HOME/.local/opt/demerzel/install.sh" uninstall
```

Explicit data removal is a separate gated operation:

```sh
bash "$HOME/.local/opt/demerzel/install.sh" uninstall --purge
```

Purge accepts only an app-owned `.demerzel` or `demerzel` directory and requires typing `PURGE` followed by the exact absolute path. It does not remove the external identity or data-root pin. Do not use purge as a substitute for a verified backup.

## Platform and publication status

The standalone native installer supports Linux amd64 and arm64. Windows custody and setup are separate from this guide. The release workflow currently accepts strict `v2.x.y` tags only; each tag must point to a commit on `main` and pass the protected release approval. No signed public release is available yet.