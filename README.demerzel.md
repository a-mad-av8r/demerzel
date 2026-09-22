# Demerzel — LLM gateway (multi-subscription router)

One local endpoint, many accounts per provider — pool Claude/Codex/Kimi
subscriptions and every major API (OpenRouter, Cohere, Together, GroqCloud, xAI,
Cerebras, Fireworks, Perplexity, Alibaba Coding/Token plans, Bedrock/Azure/Vertex,
TypeSafe Jev, custom endpoints). Deterministic serial failover with per-account
quota/cooldown state. Mac + Linux, brew/bunx/tar install.

**Status:** bootstrap. Plan-of-record: kos-infra `docs/kos-infra-adaptech/discovery/`
(`2026-09-21-llm-router-research.md` assessment + `2026-09-22-llm-gateway-execution-plan.md`).

## Basis

Forked from [tbphp/gpt-load](https://github.com/tbphp/gpt-load) v2
(`1f615d839338`, MIT) — itself a composition of Bifrost (execution/conversion) +
CLIProxyAPI (subscription OAuth executors) + its own credential registry/scheduler.
Attribution and notices retained (`LICENSE`, `THIRD_PARTY_NOTICES.md`,
`VENDORING.md`); upstream remote kept for rebases.

## Local-first

- Loopback-bound by default; credential vault local (macOS Keychain / Linux Secret
  Service; age-file headless fallback).
- No phone-home, no self-update in core; updates come from the installer
  frontends only (brew tap / bunx CLI), against a cosign-signed release manifest.
