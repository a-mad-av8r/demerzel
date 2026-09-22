# VENDORING.md — provenance ledger

**Rule (plan §2/M1):** every vendored module gets one entry here — donor repo,
immutable commit, source paths, destination module, SPDX license, our
modifications, retained notices. Per-entry, not per-repo. No code lands without a
row. Retain upstream LICENSE files; add Apache NOTICE where required.

| Donor | Commit | Source paths | Destination | SPDX | Modifications | Notices |
|---|---|---|---|---|---|---|
| tbphp/gpt-load | `1f615d839338` | whole repo (fork baseline) | project root | MIT | fork point; remote `origin`=github.com/a-mad-av8r/demerzel, `upstream`=tbphp/gpt-load for rebases | LICENSE + THIRD_PARTY_NOTICES.md retained at root |
| router-for-me/CLIProxyAPI | `ffe6ad3c5fcf` | (already vendored inside gpt-load v2 as pinned dependency) | OAuth executors/selectors via gpt-load's adapter seam | MIT | none yet | covered by gpt-load THIRD_PARTY_NOTICES |
| maximhq/bifrost | `40c3f7ee3a1a` | (already vendored inside gpt-load v2 as pinned dependency) | provider execution/conversion via gpt-load | Apache-2.0 | none yet | covered by gpt-load THIRD_PARTY_NOTICES |

New vendored code (beyond the fork baseline) MUST append a row in the same commit.

**NOTICE rule (Apache-2.0 donors):** any Apache-2.0 donor's `NOTICE` file must be
carried into this fork's top-level `NOTICE` (or its content appended) whenever its
code is present in a built artifact. Verify on every donor bump: if the donor ships
a NOTICE, ours must reflect it before release. MIT donors have no NOTICE clause;
Apache-2.0 donors do.
