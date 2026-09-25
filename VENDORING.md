# VENDORING.md — provenance ledger

**Rule (plan §2/M1):** every vendored module gets one entry here — donor repo,
immutable commit, source paths, destination module, SPDX licence, our
modifications, retained notices. Per-entry, not per-repository. No code lands
without a row. Retain upstream `LICENSE` files; add Apache `NOTICE` where required.

| Donor | Immutable revision | Source paths | Destination | SPDX | Modifications | Notices |
|---|---|---|---|---|---|---|
| tbphp/gpt-load | `1f615d839338` | entire repository (fork baseline) | project root | MIT | owned fork, `upstream` remote retained | `LICENSE`, `THIRD_PARTY_NOTICES.md`, `LICENSES/` retained |
| router-for-me/CLIProxyAPI | tag `v7.3.6` (`8c664b2fede5c83b919be1df9b01057ec4e4c950`) | `github.com/router-for-me/CLIProxyAPI/v7` plus embedded execution subset | `third_party/cpaembedded` (local replacement module) + `internal/execution/cpa` | MIT | Demerzel-owned execution-only adapter; storage, routing, quotas and refresh lifecycle owned by this fork, not the donor default store; explanatory comments translated into British English only | `THIRD_PARTY_NOTICES.md`, `LICENSES/MIT.txt` |
| maximhq/bifrost | tag `core/v1.9.0` (`b7601eb126dd5c919ea52fccce4fc32f1434342d`) | `github.com/maximhq/bifrost/core` Go module | module dependency, adapted at `internal/execution/bifrost` | Apache-2.0 | Demerzel provider conversion adapter; no direct source copy | `THIRD_PARTY_NOTICES.md`, `LICENSES/Apache-2.0.txt` (upstream pinned commit has no root NOTICE) |
| keybase/go-keychain | tag `v0.0.1` (`dd79abb5f55f5239037126b5943c0b1335a84abc`) | `github.com/keybase/go-keychain` Go module | native macOS secret custody in `internal/platform/encryption` | MIT | linked library, no source copy; Demerzel owns key identity and recovery behaviour | `THIRD_PARTY_NOTICES.md`, `LICENSES/MIT.txt` |
| keybase/dbus | commit `5aa21ea2c23a538784857945d7454f9bf83b7170` (module `v0.0.0-20220506165403-5aa21ea2c23a`) | `github.com/keybase/dbus` Go module | Linux Secret Service transport in `internal/platform/encryption` | BSD-2-Clause | linked D-Bus client, no source copy; session-bus address is supplied by the host | `THIRD_PARTY_NOTICES.md`, `LICENSES/BSD-2-Clause-dbus.txt` |
| FiloSottile/age | tag `v1.3.2` (`b74dce4cdbe35b5e5f66c06d9612b72f89028758`) | `filippo.io/age` Go module | explicit headless secret custody in `internal/platform/encryption` | BSD-3-Clause | linked library, no source copy; Demerzel owns file format and identity-path policy | `THIRD_PARTY_NOTICES.md`, `LICENSES/BSD-3-Clause-age.txt` |

New vendored code (beyond the fork baseline) MUST append a row in the same commit.

**NOTICE rule (Apache-2.0 donors):** any Apache-2.0 donor's `NOTICE` file must be
carried into this fork's top-level `NOTICE` (or its content appended) whenever its
code is present in a built artifact. Verify on every donor bump: if the donor ships
a NOTICE, ours must reflect it before release. MIT donors have no NOTICE clause;
Apache-2.0 donors do.
