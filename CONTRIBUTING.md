# Contributing

Thanks for helping improve Demerzel. This document covers the engineering rules contributors need.

## Before you start

- **Never open a public issue for a vulnerability** — use [private vulnerability reporting](https://github.com/a-mad-av8r/demerzel/security/advisories/new); see [SECURITY.md](SECURITY.md).
- For usage questions, check the [README](README.md) and [Demerzel documentation](docs/demerzel/). Discuss substantial changes in an issue first.

## Release versioning

Demerzel releases are independent of upstream GPT-Load. Use the exact Demerzel version tag or commit when reporting issues; `tbphp/gpt-load` releases do not install this fork.

The current release workflow accepts strict SemVer `v2.x` tags only. A release tag must point to a commit already on `main` and receive the matching protected-environment approval.

## Local development

Requires Go (see `go.mod`) and Node.js with pnpm (invoked through corepack).

```bash
make dev     # Build the web UI and run with race detection
make build   # Build the UI and the binary
make test    # Run Go unit tests
make check   # Run the full acceptance gate
```

`make check` covers gofmt, `go mod tidy -diff`, `go vet`, web lint / format / build, the Go build, and the full unit test suite.

`third_party/cpaembedded` is a separate Go module and is **not covered by `make check`**. When changing it, also run:

```bash
cd third_party/cpaembedded
go mod tidy -diff
go vet ./...
```

Race tests for that module run in CI; per repository convention they are not run locally.

## Submitting a pull request

1. Branch from `main`, keep the change focused, and avoid unrelated refactors or reformatting.
2. For bug fixes and behaviour changes, add a test that reproduces the problem first.
3. Run `make check` before submitting; if you cannot, say why and what remains unverified.
4. Fill in `.github/pull_request_template.md` honestly, including the checklist.
5. For user-visible changes, update the root `README.md` and the relevant `docs/` pages.

## Commit convention

Format: `<type>(scope): <summary>`, with an optional scope. Common types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`.

```text
fix(subscription): prevent a transient refresh failure from locking credentials
feat(gateway): support the OpenAI Embeddings API
```

## Code style

- UTF-8, LF endings, trailing newline; Tabs in `.go` and `Makefile`, 2 spaces elsewhere. `.editorconfig` is authoritative.
- Go code is gofmt-formatted; imports are grouped stdlib → third-party → `gpt-load/internal/...`.
- Identifiers and comments are in British English.
- User-facing error messages go through `internal/platform/i18n`, never hardcoded in handlers.

## Dependencies and security

- **New dependencies need justification** in an issue or PR, along with compatible licences.
- Never include `AUTH_KEY`, `ENCRYPTION_KEY`, upstream keys, or any real credential in commits, logs, fixtures, or screenshots.

## Code of conduct

By participating, you agree to abide by the [Code of Conduct](CODE_OF_CONDUCT.md).
