# Third-Party Notices

Demerzel, a fork of GPT-Load, includes third-party open-source software. This
file covers the components that require specific attribution, carry obligations
beyond attribution, or are modified by Demerzel. Each release also ships a
CycloneDX SBOM (`bom.cdx.json`) inventorying the resolved Go module graph.

## Bifrost Core

- Module: `github.com/maximhq/bifrost/core`
- Version: `v1.9.0` (`core/v1.9.0`, commit `b7601eb126dd5c919ea52fccce4fc32f1434342d`)
- Copyright: 2025 H3 Labs Inc.
- Licence: Apache License 2.0

Demerzel uses Bifrost Core as an infrastructure adapter for provider execution
and protocol conversion. Demerzel's domain models, persisted channel IDs,
scheduling, retry policy, health state, usage accounting, and pricing remain
owned by Demerzel.

The complete Apache License 2.0 text is distributed in
`LICENSES/Apache-2.0.txt`.

## CLIProxyAPI

- Module: `github.com/router-for-me/CLIProxyAPI/v7`
- Version: `v7.3.6`
- Copyright: 2025-2005.9 Luis Pater; 2025.9-present Router-For.ME
- Licence: MIT License

Demerzel uses a pinned, execution-only embedded adapter around CLIProxyAPI's
Codex, Claude, Antigravity and xAI OAuth and HTTP executor code. Demerzel
retains ownership of credential storage, account selection, retry, health,
affinity, logging and usage policy. The embedded adapter does not use
CLIProxyAPI's manager, account pool or file store. A separate, explicitly called
Codex WebSocket session facade reuses the pinned WebSocket executor with HTTP
fallback and blocks business-request replay. The SDK can still attempt an extra
handshake after a failed send; the facade rejects replacement connection binding
before another business request is sent. The facade is not connected to the
existing HTTP data plane.

The complete MIT License text is distributed in `LICENSES/MIT.txt`.

## macOS Keychain library

- Module: `github.com/keybase/go-keychain`
- Version: `v0.0.1` (commit `dd79abb5f55f5239037126b5943c0b1335a84abc`)
- Copyright: 2015 Keybase
- Licence: MIT License

Demerzel uses this library to store its master encryption key in the macOS
Keychain; its own code determines the account identity and import procedure.
The complete licence and copyright are in `LICENSES/MIT.txt`.

## Linux Secret Service D-Bus client

- Module: `github.com/keybase/dbus`
- Version: `v0.0.0-20220506165403-5aa21ea2c23a` (commit `5aa21ea2c23a538784857945d7454f9bf83b7170`)
- Copyright: 2013 Georg Reinke and Google
- Licence: BSD-2-Clause

Demerzel uses this local session-bus client to access the Secret Service
collection on Linux. The complete licence is in `LICENSES/BSD-2-Clause-dbus.txt`.

## age

- Module: `filippo.io/age`
- Version: `v1.3.2` (commit `b74dce4cdbe35b5e5f66c06d9612b72f89028758`)
- Copyright: 2019 The age Authors; 2019 Google LLC; 2022 Filippo Valsorda
- Licence: BSD-3-Clause

Demerzel uses age for its explicitly configured headless master-key recovery
file. The complete licence and copyright are in `LICENSES/BSD-3-Clause-age.txt`.

## Inno Setup (Windows installer)

- Component: Inno Setup 6, used by the Windows packaging workflow
- Copyright: 1997-2026 Jordan Russell; 2000-2026 Martijn Laan
- Licence: Inno Setup License

Demerzel's Windows installer uses the English messages supplied by the Inno
Setup compiler. The compiler is a build-time packaging dependency.

The complete Inno Setup License text is distributed in
`LICENSES/Inno-Setup.txt`.

## fasthttp

- Module: `github.com/valyala/fasthttp`
- Version: `v1.74.0`
- Copyright: 2015-present Aliaksandr Valialkin, VertaMedia, Kirill Danshin, Erik
  Dubbelboer, FastHTTP Authors
- Licence: MIT License

Demerzel uses the official upstream release through Bifrost Core for provider
HTTP requests and streaming responses.

The complete MIT License text is distributed in `LICENSES/MIT.txt`.

## go-brrr

- Module: `github.com/molecule-man/go-brrr`
- Version: `v1.0.1`
- Copyright: 2026 Andrii Berezhynskyi
- Licence: MIT License

Demerzel includes go-brrr through fasthttp for Brotli compression and
decompression.

The complete MIT License text is distributed in `LICENSES/MIT.txt`.

## Go MySQL Driver

- Module: `github.com/go-sql-driver/mysql`
- Version: `v1.8.1`
- Copyright: 2012 The Go-MySQL-Driver Authors
- Licence: Mozilla Public License 2.0

Linked unmodified, through `gorm.io/driver/mysql`, for MySQL support. As required
by MPL-2.0 Section 3.2, the Source Code Form for this version is available under
the terms of the MPL at
<https://github.com/go-sql-driver/mysql/tree/v1.8.1>.

The complete Mozilla Public License 2.0 text is distributed in
`LICENSES/MPL-2.0.txt`.

## Lobe Icons

- Source: `@lobehub/icons-static-svg` `1.94.0` (vendored subset, not an npm
  dependency of the management UI)
- Copyright: 2023 LobeHub
- Licence: MIT License

Demerzel vendors a subset of Lobe Icons' SVG marks (`web/src/assets/channels/`)
to identify built-in channel presets by their upstream provider's brand in the
management UI. The vendored icons and this notice do not grant any trademark
rights in the marks they depict.

The complete MIT License text is distributed in `LICENSES/MIT.txt`.
