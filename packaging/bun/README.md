# Bun launcher

This package provides a Bun-compatible command-line wrapper for the Demerzel Go gateway. Bun runs the launcher; the gateway itself remains a Go executable. The wrapper downloads the version-matched native release binary only after verifying the release manifest's Sigstore bundle and the binary's SHA-256 digest.

## Publication status

The package is prepared locally as `@amadmalik/demerzel` version `2.0.0`, but it has not been published to npm. The matching signed GitHub release is also not available, so invoking the gateway currently fails closed with a clear release-not-published message. Do not treat the package as an available install channel until both are published through their approval gates.

The release workflow currently accepts strict `v2.x.y` SemVer tags. The wrapper reads its version from this package's `package.json` and requires the corresponding `v<version>` release in `a-mad-av8r/demerzel`.

## Local checks

From the repository root:

```sh
bun packaging/bun/bin/demerzel.mjs --help
bun packaging/bun/bin/demerzel.mjs --version
```

These commands do not access the network. Starting the gateway requires a published signed release, a supported macOS or Linux target (amd64 or arm64), and `cosign` available on `PATH`.

## Use after publication

After this package and its matching signed GitHub release are published:

```sh
bunx --bun @amadmalik/demerzel@2.0.0
```

Pass normal Demerzel command-line arguments after the package name. The launcher verifies the release manifest against the Demerzel GitHub Actions release workflow identity and GitHub Actions OIDC issuer, then checks the downloaded binary against the signed SHA-256 digest before executing it.

The verified binary is cached under `${XDG_CACHE_HOME:-~/.cache}/demerzel/bun/<version>` with owner-only permissions. The launcher does not store credentials. Configure Demerzel's runtime and key custody through the same environment and operating procedures as the Go binary; see the [installation and custody guides](../../docs/demerzel/install-macos.md).

Supported native targets: macOS and Linux, x64/amd64 and arm64. Other operating systems and architectures fail closed.
