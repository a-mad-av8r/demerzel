# Demerzel

![Demerzel — local-first AI gateway](web/public/assets/demerzel_logo.png)

**A local-first gateway for managing provider accounts, model access and failover.**

Demerzel is distributed under the MIT Licence and is forked from [tbphp/gpt-load v2](https://github.com/tbphp/gpt-load/tree/1f615d839338); see [`LICENSE`](LICENSE), [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) and [`VENDORING.md`](VENDORING.md) for attribution and component details.

## What it does

- **Manages multiple provider accounts.** Keep credentials individually named and encrypted; enable or disable accounts without changing clients.
- **Separates model choice from account choice.** Configure channel protocols and model names independently from the account-selection strategy.
- **Supports per-group routing.** New groups default to serial account selection; existing groups that omit the setting retain weighted-fair selection. A quota reserve can be configured from 0 to 100 per cent.
- **Fails over predictably.** Credential-scoped quota rejections advance the persisted account cursor. Model- or request-scoped limits do not move it; replay-safe requests may use another account for that request.
- **Provides OpenAI-compatible endpoints.** Clients can use the supported Chat Completions and Responses protocols; Anthropic and Gemini adapters are also available.
- **Includes a management UI and account CLI.** The browser UI manages channels, groups, keys and usage. Secret-bearing account commands use an owner-only Unix socket rather than a plain HTTP control endpoint.
- **Connects OMP and OpenKai.** The home page generates OpenAI Responses-compatible provider configuration for both clients. Choose an access key that supports `openai-responses` and a model; keep `DEMERZEL_API_KEY` in the client's environment file, not in `models.yml`.

See the [visual architecture](docs/demerzel/architecture.html), [module contracts](docs/demerzel/module-contracts.md), [credential accounts guide](docs/demerzel/accounts.md) and [key-custody runbook](docs/demerzel/key-custody.md).

## Availability and installation

The source is public. **No signed GitHub release, Homebrew tap or Bun/npm package has been published.** Pre-built `curl` and Homebrew installation therefore cannot be used yet. The release workflow also requires an approved tag on `main` and protected release approval; it accepts stable `v2.x.y` tags and prereleases that pass its version validation, but not build metadata.

| Channel                   | Current state                                                                                                  | Guide                                                                             |
| ------------------------- | -------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| Build from source         | Available from a checkout containing this revision                                                             | Instructions below                                                                |
| Signed release via `curl` | Installer and verification procedure are ready; release assets are not published                               | [macOS](docs/demerzel/install-macos.md) · [Linux](docs/demerzel/install-linux.md) |
| Homebrew                  | Formula is in `packaging/homebrew/demerzel.rb`; the public tap and release are not available                   | [macOS installation](docs/demerzel/install-macos.md)                              |
| Bun                       | A Bun-compatible launcher for the Go gateway is prepared locally; package and signed release are not published | [Bun launcher](packaging/bun/README.md)                                           |
| Rootless Podman           | Local container build and run instructions are available                                                       | [Container installation](docs/demerzel/install-containers.md)                     |

### Build and run from source

Requirements: Go 1.27, Node.js 24.11 or later, Corepack and pnpm 11.17. The
current public implementation branch is `adam/m2-productization`; the source
build commands below start from that branch.

```sh
git clone --branch adam/m2-productization --single-branch \
  https://github.com/a-mad-av8r/demerzel.git
cd demerzel
corepack enable
make build
DATA_DIR="$HOME/.local/share/demerzel-dev" ./demerzel
```

Open <http://127.0.0.1:3001>. The server binds to loopback by default. Use a dedicated `DATA_DIR` for development; do not point a fresh key at an existing database or move a populated data directory without following the [custody and restore procedure](docs/demerzel/key-custody.md).

The first run creates management authentication material in `DATA_DIR/auth.key`. Keep the data directory owner-only. For unattended hosts, configure explicit age custody before first boot; desktop macOS and Linux may use their unlocked native secret services. Windows does not currently provide those custody backends and requires `ENCRYPTION_KEY` from a controlled environment.

### Connecting OMP and OpenKai

In the home page, choose **OMP** or **OpenKai**, select an access key that permits the `openai-responses` protocol, and choose a model exposed by that key. Merge the generated `demerzel` provider entry into the existing `~/.omp/agent/models.yml`; do not replace other providers. Both clients use that OMP-compatible model file by default.

Store the key as `DEMERZEL_API_KEY` in `~/.omp/agent/.env` for OMP or `~/.openkai/.env` for OpenKai. Keep those files owner-only. The generated model YAML contains only the environment-variable name; the key is copied separately or included only when the explicit environment-file copy action is used.

## Runtime and recovery

- **Listener:** loopback HTTP on port 3001 by default. Do not expose it directly to an untrusted network.
- **Data:** use one canonical absolute `DATA_DIR` for every restart, upgrade and rollback. macOS defaults to `~/.demerzel`; Linux defaults to `${XDG_DATA_HOME:-$HOME/.local/share}/demerzel`.
- **Age custody:** unattended native installations keep the age identity at `$HOME/.config/demerzel/identity.txt`, outside `DATA_DIR`. The standalone and Homebrew installers pin the selected absolute data path and fail closed when the matching identity is missing.
- **Account CLI:** run it as the gateway's Unix user with the same `DATA_DIR`; it connects through `DATA_DIR/control.sock` and does not use the browser listener as a control transport.
- **Backups:** preserve the database and SQLite sidecars when applicable, `auth.key`, the matching native custody item or the external age identity plus `encryption.key.age`, `runtime-state.checkpoint.json` and operator configuration as one recovery set. Verify a restore before relying on it.
- **Uninstall:** package removal preserves runtime data by default. Explicit purge requires a separate exact-path confirmation and never removes the external age identity.

The non-secret vault locator can be recorded with backup metadata:

```sh
demerzel key-locator --data-dir "${DATA_DIR:-$(cat "$HOME/.config/demerzel/data-dir")}"
```

The locator is not key material and does not prove a restore. Follow the [key-custody runbook](docs/demerzel/key-custody.md) before changing custody or moving data.

## Account routing

New groups persist `overrides.account_selection=serial`; existing groups without the setting retain `weighted_fair`. This per-group setting does not change the global `route_strategy`. Serial selection uses the configured quota reserve only when an account-level observation supplies both a remaining ratio and reset time.

A credential-scoped quota rejection blocks the primary account; the next eligible selection advances and persists the account cursor. Model- and request-scoped limits do not move that cursor. Failback waits 60 seconds after the blocked window resets and retries with back-off from 5 seconds to 5 minutes. A provider-declared, idempotent list-models GET can be a recovery probe; otherwise the next caller-initiated inference is used. Only a successful caller inference promotes the primary account. The scheduling checkpoint retains cursor and back-off state across clean restarts.

## Development

```sh
make dev       # Build the web UI and run the application with Go's race detector
make build     # Build the web UI and the Demerzel binary
make test      # Run Go unit tests
make check     # Run formatting, static checks, web validation, build and Go tests
```

`third_party/cpaembedded` is a separate Go module. When changing it, follow the extra checks in [`CONTRIBUTING.md`](CONTRIBUTING.md). For local Podman setup, external age custody and persistent-volume recovery, see the [container guide](docs/demerzel/install-containers.md).

## Project documents

- [macOS install and Homebrew status](docs/demerzel/install-macos.md)
- [Linux install and signed `curl` procedure](docs/demerzel/install-linux.md)
- [Credential accounts](docs/demerzel/accounts.md)
- [Key custody and recovery](docs/demerzel/key-custody.md)
- [Module contracts](docs/demerzel/module-contracts.md)
- [Visual architecture](docs/demerzel/architecture.html)
- [Third-party notices](THIRD_PARTY_NOTICES.md)
- [Vendoring and provenance](VENDORING.md)
- [Security reporting](SECURITY.md)

## Licence

Demerzel is distributed under the MIT Licence. It retains the upstream GPT-Load fork history and attribution; third-party licences and modifications are listed in [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) and [`VENDORING.md`](VENDORING.md).
