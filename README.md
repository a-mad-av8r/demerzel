# Demerzel

**Status: installer productization is under review. No release, package, or Homebrew tap has been published.**

Demerzel is a local-first LLM gateway forked from
[tbphp/gpt-load v2 @ `1f615d83`](https://github.com/tbphp/gpt-load/tree/1f615d839338)
(MIT). It provides individually named provider accounts, model selection
separate from account selection, and deterministic serial failover.

- [macOS installation](docs/demerzel/install-macos.md) — private Homebrew formula, service custody, state and rollback.
- [Linux installation](docs/demerzel/install-linux.md) — authenticated artifact download and persistent XDG paths.
- [Rootless Podman installation](docs/demerzel/install-containers.md) — local Demerzel build, named data volume, external age identity.
- [Key custody and recovery](docs/demerzel/key-custody.md) — native vaults, age custody, migration and restore boundaries.
- [Actual Go module contracts](docs/demerzel/module-contracts.md) — owned interfaces and conformance checks.
- [Credential accounts](docs/demerzel/accounts.md) — supported account CLI workflows and protected credential output.
- [Visual architecture](docs/demerzel/architecture.html) — open locally in any browser.
- [Donor provenance](VENDORING.md) — exact upstream module revisions, modifications and retained notices.
- [Switchyard spike](docs/demerzel/2026-09-24-switchyard-spike.md) — evaluation only; the benchmark is blocked pending authorized provider responses and raw p95 data. It is not measured performance or released model policy.
- [Research and execution plan](https://github.com/Adaptech-AI/kos-infra/tree/adam/llm-router-research/docs/kos-infra-adaptech/discovery) — design decisions and release gates.

## Private installation status

`packaging/homebrew/demerzel.rb` is the tap-ready macOS formula. The intended
single Homebrew command, after the private tap and a signed release are approved,
is:

```sh
HOMEBREW_GITHUB_API_TOKEN="$(gh auth token)" brew install a-mad-av8r/tap/demerzel
```

It is **not usable yet**: the tap repository and release assets are publication
gates and have not been created or published by this work. Authenticate to the
private GitHub repository explicitly with `gh auth login --hostname
github.com --scopes repo` first. The token is passed through the environment, not
printed or embedded in a formula. The formula verifies the Sigstore bundle
against the expected Demerzel release workflow identity and verifies the chosen
binary against the signed manifest. Do not substitute an upstream download or
an unverified `curl | sh` command.

Demerzel has no in-app release checker or updater. Update checking and install
verification belong to the Demerzel installer; the running service does not
query `tbphp/gpt-load` releases.

## Runtime and recovery invariants

- macOS Homebrew service: installed binary under the Homebrew prefix; persistent
  `DATA_DIR=$HOME/.demerzel`; external age identity at
  `$HOME/.config/demerzel/identity.txt`.
- Linux native install: persistent data under
  `${XDG_DATA_HOME:-$HOME/.local/share}/demerzel`; external identity under
  `$HOME/.config/demerzel/identity.txt`.
- Rootless Podman: Demerzel-owned local image, named `demerzel-data` volume at
  `/app/data`, and a read-only age identity bind mount outside that volume.
- Secret-bearing account CLI commands connect only through the owner-only Unix
  socket at `DATA_DIR/control.sock`, with the same Unix user and exact `DATA_DIR`
  as the gateway. The browser/data-plane HTTP listener remains loopback; it is
  not an account CLI transport.
- Keep the same canonical absolute `DATA_DIR` for every restart, upgrade and
  rollback. Its canonical path participates in native-vault custody and the
  installation identity. A database alone is not a backup.
- Preserve the database, `auth.key`, the matching Keychain/Secret Service item
  or the separate age identity plus `encryption.key.age`, and
  `runtime-state.checkpoint.json` as one recovery set. Uninstall preserves
  state by default. The explicit `--purge` path removes only the app-owned data
  directory after an exact typed confirmation; it never removes the external
  age identity.
- Unattended macOS services use age custody from first boot. Do not switch an
  existing database to another key backend. Windows has no supported native
  vault or age custody in this productization; Windows operators must provide
  `ENCRYPTION_KEY` explicitly in a controlled environment.

For native-vault escrow and recovery, record the non-secret locator alongside
backup metadata:

```sh
demerzel key-locator --data-dir "$HOME/.demerzel"
```

The locator is not key material and does not replace a tested database-plus-key
restore. See the [custody runbook](docs/demerzel/key-custody.md).

## Origin and scope

The fork keeps upstream history and attribution (`LICENSE`,
`THIRD_PARTY_NOTICES.md`, `LICENSES/`). `upstream` points to tbphp/gpt-load;
`origin` points to a-mad-av8r/demerzel. Execution uses the existing Bifrost Core
and CLIProxyAPI adapters. Do not import primary subscription accounts or treat
this work as a production release. `README_CN.md` and `README_JP.md` remain
historical upstream documentation, not Demerzel installation guides.

## Run from source (development only)

Use Go 1.27, Node ≥24.11 and pnpm 11.17. A desktop macOS development process may
use the unlocked login Keychain; Linux desktop development may use an unlocked
Secret Service. For an unattended process, configure explicit age custody
before first boot.

```sh
pnpm --dir web install --frozen-lockfile
pnpm --dir web run build
go build -o /tmp/demerzel .
DATA_DIR="$HOME/.local/share/demerzel-dev" /tmp/demerzel
```

The browser/data-plane listener is `http://127.0.0.1:3001`; keep it on loopback
unless a separately protected deployment is intentionally configured. The
secret-bearing account CLI uses only the owner-only `DATA_DIR/control.sock`
Unix socket with the same user and absolute `DATA_DIR` as the gateway; it does
not send `AUTH_KEY` or upstream keys through a plain HTTP listener. Generated
management authentication is stored in `DATA_DIR/auth.key`. This is a
development recipe, not a production service or backup system.

## Optional contributor tooling: Cortex

Demerzel runs without Cortex. Contributors who want agent memory and handoffs
can read [Kaidera-AI/cortex](https://github.com/Kaidera-AI/cortex), with
[macOS](https://github.com/Kaidera-AI/cortex/blob/main/docs/install-macos.md)
and [Linux](https://github.com/Kaidera-AI/cortex/blob/main/docs/install-linux.md)
guides. Rootless Podman is an optional contributor/deployment runtime, not a
Demerzel cloud dependency.
