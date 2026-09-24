# Demerzel

**Status: M1 hardening on `adam/m1-hardening`; no team release yet.**

Demerzel is a local-first LLM gateway forked from
[tbphp/gpt-load v2 @ `1f615d83`](https://github.com/tbphp/gpt-load/tree/1f615d839338)
(MIT). It is being developed for a single loopback endpoint with multiple
individually named accounts per provider, model selection separate from account
selection, and deterministic serial failover. macOS and Linux distribution
(Homebrew, Bun installer, signed tarballs) are later acceptance gates, **not
published installation methods today**.

- [Visual architecture](docs/demerzel/architecture.html) — open the HTML file in
  any browser; no hosted service required.
- [Actual Go module contracts](docs/demerzel/module-contracts.md) — owned
  interfaces, injection points, and conformance checks.
- [Key custody and recovery](docs/demerzel/key-custody.md) — native vaults,
  explicit age mode, guarded legacy import, and external database initialization.
- [Donor provenance](VENDORING.md) — exact upstream module revisions, modifications,
  licenses, and retained notices.
- [Research and execution plan](https://github.com/Adaptech-AI/kos-infra/tree/adam/llm-router-research/docs/kos-infra-adaptech/discovery)
  — design decisions, phases, and release gates.

## Origin and custody

The fork keeps upstream history and attribution (`LICENSE`,
`THIRD_PARTY_NOTICES.md`, `LICENSES/`). `upstream` points to tbphp/gpt-load;
`origin` points to a-mad-av8r/demerzel. Execution uses the existing Bifrost
Core and CLIProxyAPI adapters. M1 introduces native master-key custody and
default-off catalog synchronization; cross-platform and restore-proof gates
remain. **Do not import primary subscription accounts or treat this as a
production release yet.** `README_CN.md` and `README_JP.md` are upstream
gpt-load historical documentation, not Demerzel installation guides.

For unattended macOS service mode, use explicit age custody **from first boot**.
Ad-hoc rebuilt binaries can block at the Keychain ACL prompt; native-vault
upgrade/recovery still needs signed-release and restore-proof gates. See the
[custody runbook](docs/demerzel/key-custody.md).

## Run from source (development only)

Use Go 1.27, Node ≥24.11 and pnpm 11.17. On macOS, unlock the login Keychain;
on Linux, provide an unlocked desktop Secret Service or configure the [age
headless mode](docs/demerzel/key-custody.md) **before first boot**.

```sh
pnpm --dir web install --frozen-lockfile
pnpm --dir web run build
go build -o /tmp/demerzel-m1 .
DATA_DIR="$HOME/.local/share/demerzel-dev" /tmp/demerzel-m1
```

The local management page is `http://127.0.0.1:3001`; generated management
authentication lives in `DATA_DIR/auth.key`. Keep the same absolute `DATA_DIR`
and binary for ordinary restarts. This is **not** an installer or production
service recipe.

## Optional contributor tooling: Cortex

Demerzel runs without Cortex. Contributors who want agent memory and handoffs
can read [Kaidera-AI/cortex](https://github.com/Kaidera-AI/cortex), with
[macOS](https://github.com/Kaidera-AI/cortex/blob/main/docs/install-macos.md)
and [Linux](https://github.com/Kaidera-AI/cortex/blob/main/docs/install-linux.md)
guides. Its **v0.1.002 partial release** installs the npm/Bun/Homebrew *launcher*
and `preflight`; the standalone `cortex install` command currently refuses
until a digest-pinned stack payload ships (v0.1.003 work). Rootless Podman ≥5.0
is a Cortex prerequisite, not a Demerzel dependency.
