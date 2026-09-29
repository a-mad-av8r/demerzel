# Acknowledgements

Demerzel builds on open-source work by the people and projects below. This page records where its code, data and ideas came from, and keeps the provenance ledger for everything copied into the repository.

The licence notices that travel with every copy of Demerzel are in [`THIRD_PARTY_NOTICES.md`](../../../THIRD_PARTY_NOTICES.md) and [`LICENSES/`](../../../LICENSES) at the repository root. GitHub Release assets and container images include both.

## Code

| Project | People | What Demerzel uses | Licence |
|---|---|---|---|
| [GPT-Load](https://github.com/tbphp/gpt-load/tree/1f615d839338cdb97fc5d7210eb10626b51f9946) | tbphp and the GPT-Load contributors | The codebase Demerzel started from: account registry and scheduler, credential refresh lifecycle, credential encryption, management API and web interface | MIT |
| [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) | Luis Pater and Router-For.ME | Sign-in and request execution for ChatGPT (Codex), Claude, Antigravity and Grok subscriptions, through `third_party/cpaembedded` | MIT |
| [Bifrost Core](https://github.com/maximhq/bifrost) | H3 Labs Inc. | Provider request execution and protocol conversion, through `internal/execution/bifrost` | Apache-2.0 |

Libraries with specific notice obligations are listed in [`THIRD_PARTY_NOTICES.md`](../../../THIRD_PARTY_NOTICES.md). Each release also ships a software bill of materials (`bom.cdx.json`) listing every Go module in the build.

## Data and artwork

| Source | People | What Demerzel uses | Licence |
|---|---|---|---|
| [OpenAI Codex](https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/models-manager/models.json) | OpenAI | The Codex client model catalogue, embedded unmodified as `internal/catalog/codex_client_models_0.154.0.json` | Apache-2.0 |
| [models.dev](https://github.com/anomalyco/models.dev) | models.dev | Model catalogue, limits and prices, downloaded when catalogue sync runs | MIT |
| [Lobe Icons](https://github.com/lobehub/lobe-icons) | LobeHub | Provider icons in the management interface | MIT |
| [Octicons](https://github.com/primer/octicons) | GitHub | The GitHub mark in the management interface | MIT |

## Ideas and references

No code was copied from these projects; they shaped how Demerzel works.

| Project | What Demerzel took from it | Licence |
|---|---|---|
| [OmniRoute](https://github.com/diegosouzapw/OmniRoute/tree/06f1df9d775d539610bb4c22185c1174d9a0c0b5) | Account-selection strategy semantics: headroom, reset windows, quota-share scheduling | MIT |
| [Claude Relay Service (CRS)](https://github.com/Wei-Shaw/claude-relay-service/tree/cf95ecc5e0ba846faaf1b11de574367de89fd00c) | Claude subscription patterns: 5-hour and weekly windows, reset cooldowns, account pools | MIT |
| [grok2api](https://github.com/chenyme/grok2api/tree/5e5ad75556b61a2c4a8fcf344d83bfe7760f2b42) | Grok account-pool domain and ranking patterns | MIT |
| [NVIDIA NeMo Switchyard](https://github.com/NVIDIA-NeMo/Switchyard/tree/9cf6fadfc60bfdf59ee61bf8a14226807c2540bc) | Model-routing patterns; the [Switchyard spike](../2026-09-24-switchyard-spike.md) used it as an evaluation reference | Apache-2.0 |

## Provenance ledger

**Rule (plan §2/M1):** every vendored module or copied file gets one entry here: donor repository, immutable revision, source paths, destination, SPDX licence, Demerzel's modifications and retained notices. Per entry, not per repository. No code lands without a row, and new vendored code appends its row in the same commit. Keep upstream `LICENSE` files, and keep each component's licence notice in `THIRD_PARTY_NOTICES.md` and `LICENSES/`.

| Donor | Immutable revision | Source paths | Destination | SPDX | Modifications | Notices |
|---|---|---|---|---|---|---|
| tbphp/gpt-load | `1f615d839338cdb97fc5d7210eb10626b51f9946` | entire repository (starting codebase) | project root | MIT | Demerzel-owned codebase; `upstream` remote retained for deliberate merges | `THIRD_PARTY_NOTICES.md`, `LICENSES/MIT.txt` |
| router-for-me/CLIProxyAPI | tag `v7.3.6` (`8c664b2fede5c83b919be1df9b01057ec4e4c950`) | `github.com/router-for-me/CLIProxyAPI/v7` plus embedded execution subset | `third_party/cpaembedded` (local replacement module) + `internal/execution/cpa` | MIT | Demerzel-owned execution-only adapter; storage, routing, quotas and refresh lifecycle owned by Demerzel, not the donor default store; explanatory comments translated into British English only | `THIRD_PARTY_NOTICES.md`, `LICENSES/MIT.txt` |
| maximhq/bifrost | tag `core/v1.9.0` (`b7601eb126dd5c919ea52fccce4fc32f1434342d`) | `github.com/maximhq/bifrost/core` Go module | module dependency, adapted at `internal/execution/bifrost` | Apache-2.0 | Demerzel provider conversion adapter; no direct source copy | `THIRD_PARTY_NOTICES.md`, `LICENSES/Apache-2.0.txt` (upstream pinned commit has no root NOTICE) |
| keybase/go-keychain | tag `v0.0.1` (`dd79abb5f55f5239037126b5943c0b1335a84abc`) | `github.com/keybase/go-keychain` Go module | native macOS secret custody in `internal/platform/encryption` | MIT | linked library, no source copy; Demerzel owns key identity and recovery behaviour | `THIRD_PARTY_NOTICES.md`, `LICENSES/MIT.txt` |
| keybase/dbus | commit `5aa21ea2c23a538784857945d7454f9bf83b7170` (module `v0.0.0-20220506165403-5aa21ea2c23a`) | `github.com/keybase/dbus` Go module | Linux Secret Service transport in `internal/platform/encryption` | BSD-2-Clause | linked D-Bus client, no source copy; session-bus address is supplied by the host | `THIRD_PARTY_NOTICES.md`, `LICENSES/BSD-2-Clause-dbus.txt` |
| FiloSottile/age | tag `v1.3.2` (`b74dce4cdbe35b5e5f66c06d9612b72f89028758`) | `filippo.io/age` Go module | explicit headless secret custody in `internal/platform/encryption` | BSD-3-Clause | linked library, no source copy; Demerzel owns file format and identity-path policy | `THIRD_PARTY_NOTICES.md`, `LICENSES/BSD-3-Clause-age.txt` |
| openai/codex | tag `rust-v0.154.0` (`6b9826e3aa83b1a5947db50f4332cb9c65f1b340`) | `codex-rs/models-manager/models.json` | `internal/catalog/codex_client_models_0.154.0.json` | Apache-2.0 | none; byte-identical copy (blob `c9b4d6ce6e85acc87c236e83421e3b4520e1a5a0`) | `THIRD_PARTY_NOTICES.md` (with the upstream NOTICE attribution), `LICENSES/Apache-2.0.txt` |
| lobehub/lobe-icons | npm `@lobehub/icons-static-svg` `1.94.0` | provider SVG marks (vendored subset) | `web/src/frontends/classic/assets/channels/`, `web/src/frontends/modern/assets/channels/` | MIT | subset only; not an npm dependency of the management UI | `THIRD_PARTY_NOTICES.md`, `LICENSES/MIT.txt` |
| primer/octicons | not recorded (icon `mark-github-16`) | `icons/mark-github-16.svg` | `web/src/frontends/modern/components/GitHubIcon.vue` | MIT | SVG path inlined into a Vue component using `currentColor` | `THIRD_PARTY_NOTICES.md`, `LICENSES/MIT.txt`, `web/src/frontends/modern/assets/brand/octicons.LICENSE` |

**NOTICE rule (Apache-2.0 donors):** when an Apache-2.0 donor ships a `NOTICE` file, reproduce its attribution notices that apply to Demerzel in `THIRD_PARTY_NOTICES.md`, which ships with every built artifact, whenever the donor's code or data is present in that artifact. Check again on every donor bump. MIT donors have no NOTICE clause; Apache-2.0 donors do.
