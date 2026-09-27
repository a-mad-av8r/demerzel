<p align="center">
  <img src=".github/assets/demerzel-logo.png" alt="Demerzel: intelligence with a soul" width="340">
</p>

<p align="center">
  <strong>A local-first AI gateway for your provider accounts, models, usage and failover.</strong>
</p>

<p align="center">
  <a href="https://github.com/a-mad-av8r/demerzel/tree/adam/m2-productization"><strong>Source</strong></a> ·
  <a href="#install-from-source"><strong>Install</strong></a> ·
  <a href="#documentation"><strong>Documentation</strong></a> ·
  <a href="#roadmap"><strong>Roadmap</strong></a>
</p>

<p align="center">
  <img alt="Status: pre-release" src="https://img.shields.io/badge/status-pre--release-c9a45c">
  <a href="https://github.com/a-mad-av8r/demerzel/blob/main/LICENSE"><img alt="Licence: MIT" src="https://img.shields.io/badge/licence-MIT-2f2a24"></a>
</p>

Demerzel runs on your own machine, between your AI harnesses and the model providers you pay for. Point [OpenKai](https://github.com/Kaidera-AI/OpenKai), [OMP](https://github.com/can1357/oh-my-pi) and the many other harnesses you use at one local endpoint. Keep every provider account in one encrypted place, give each harness its own access key, and let Demerzel route each request to the right provider and account, record usage and move to the next account when one runs out of quota.

## Opt-in by design

In Asimov's *Foundation*, Demerzel steers the Empire from behind the throne. This Demerzel does the opposite: it only handles what you send it.

- **Your harnesses keep their own logins and defaults.** You choose Demerzel per request, for example by picking a `demerzel/…` model in OpenKai, OMP or any other harness that lets you add a custom provider. Stopping Demerzel returns everything to direct use.
- **Local by default.** The gateway listens on loopback only, so nothing else on your network can reach it unless you choose to expose it.

> [!NOTE]
> **Pre-release:** the current build does not fully meet this goal yet. Its container publishes OAuth callback ports that some tools need for their own sign-in, and a few client set-up snippets switch a tool's default provider. Both are being fixed; see the [roadmap](#roadmap).

## What it does

- **One endpoint for every harness.** Point OpenKai, OMP and any other harness that accepts a custom OpenAI, Anthropic or Gemini endpoint at a single local address. It speaks OpenAI Chat Completions and Responses, with Anthropic and Gemini adapters.
- **Encrypted account vault.** Each credential is individually named and encrypted at rest. The master key is held by your operating system's keychain or secret service, or by an external age identity.
- **Model choice separate from account choice.** Map channel protocols and model names independently of which account serves them.
- **Predictable failover.** When an account hits its quota, Demerzel moves to the next one, remembers its place across restarts, and returns to the primary account once it recovers.
- **Per-group routing.** Choose serial or weighted-fair account selection for each group, with an optional quota reserve.
- **Management UI and account CLI.** Manage channels, groups, access keys and usage in the browser. Secret-bearing account commands use an owner-only Unix socket, never plain HTTP.

## Providers

| API-key providers | Subscription sign-in |
| --- | --- |
| OpenAI · Anthropic · Google Gemini · Google Vertex AI · AWS Bedrock · Azure OpenAI · Alibaba Cloud (Qwen) · Moonshot AI (Kimi) · DeepSeek · Zhipu AI (GLM) · Groq · xAI · OpenRouter · SiliconFlow · Volcengine · TypeSafe Jev · any OpenAI-compatible API · other gateways (New API, CLIProxyAPI, Sub2API) | ChatGPT (Codex) · Claude · Google Antigravity · Grok |

Some providers restrict using a consumer subscription outside their own apps. Demerzel shows a risk notice when you add a Claude or Antigravity subscription; you are responsible for following each provider's terms.

## Install from source

There is no signed release, Homebrew tap or package yet. Until the first release reaches `main`, the source lives on the [`adam/m2-productization`](https://github.com/a-mad-av8r/demerzel/tree/adam/m2-productization) branch.

Requirements: Go 1.27, Node.js 24.11 or later, Corepack and pnpm 11.17.

```sh
git clone --branch adam/m2-productization --single-branch https://github.com/a-mad-av8r/demerzel.git
cd demerzel
corepack enable
make build
DATA_DIR="$HOME/.local/share/demerzel-dev" ./demerzel
```

Open <http://127.0.0.1:3001>. The first run creates management credentials in `DATA_DIR/auth.key`, so keep the data directory owner-only. Before moving data or changing key custody, read the [key custody and recovery guide](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/docs/demerzel/key-custody.md).

## Documentation

- Install on [macOS](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/docs/demerzel/install-macos.md), [Linux](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/docs/demerzel/install-linux.md) or [rootless Podman](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/docs/demerzel/install-containers.md)
- [Credential accounts](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/docs/demerzel/accounts.md)
- [Key custody and recovery](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/docs/demerzel/key-custody.md)
- [Module contracts](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/docs/demerzel/module-contracts.md)
- [Architecture](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/docs/demerzel/architecture.html) (an HTML page: download it and open it in a browser)
- [Contributing](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/CONTRIBUTING.md) · [Security policy](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/SECURITY.md) · [Third-party notices](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/THIRD_PARTY_NOTICES.md)

## Roadmap

- **Never block other tools' sign-in.** Stop publishing the fixed OAuth callback ports that native tools use (such as 1455, 54545 and 51121) and sign in with device codes or a pasted callback instead. The gateway's own port moves from 3001 to 4474.
- **Guided set-up for any harness.** Connection snippets beyond OpenKai and OMP, each adding a separate Demerzel entry and explaining how to go back to direct use.
- **Routing by remaining quota and speed.** Send each request to the account with the most tokens left, and reroute when a provider can no longer serve it or its token generation becomes too slow.
- **Any subscription, per key.** More subscription sign-ins, such as Alibaba plans and Kimi Code, plus per-access-key control over which subscription and API-key accounts each client can reach.
- **Signed releases.** macOS and Linux installers through `curl`, Homebrew and Bun, built from approved tags on `main`.

## Licence

Demerzel is released under the [MIT Licence](LICENSE).

Please report security issues privately, as described in the [security policy](https://github.com/a-mad-av8r/demerzel/blob/adam/m2-productization/SECURITY.md).
