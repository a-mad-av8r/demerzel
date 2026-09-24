# Demerzel module contracts (M1)

Demerzel is a compiled Go service, not a runtime plugin host. “Replaceable” means
an implementation behind an owned interface can change without changing the
request contract or persisted account identity; a replacement is built, tested,
and released. There is no hot-plugging arbitrary code into the running binary.

| Boundary | Existing contract / implementation | Replacement check |
|---|---|---|
| Provider execution | `internal/execution.Executor` owns `Execute`/`ExecuteStream` for exactly one preselected `AttemptSpec`. `internal/provideradapter.Registry` binds `ProviderKind` to an executor and rejects missing or unsupported route capabilities at compile time. Actual Bifrost and CPA implementations are wired by `internal/container/container.go`. | `TestRegistryDispatchesOnlyByCompiledChannelProviderBinding` and `TestRegistryRequiresAdaptersToValidateDeclaredCapabilities`; a new implementation must support both stream and unary, declare route capabilities and preserve the `AttemptResult` error/replay contract. |
| Credential eligibility | `internal/scheduler.CredentialSource` supplies credential metadata and shared scheduling state; no plaintext credential reaches the scheduler. `internal/scheduler.Iterator` implements `SelectionIterator`. | `TestConfiguredSelectorChangesWhichAccountServesRequest` proves a different selection factory changes the credential of a real gateway request; nil iterator fails without upstream dispatch. Configure the factory **before** serving requests. |
| Secret cryptography | `internal/platform/encryption.Service` owns Encrypt/Decrypt/Hash and keeps AES-256-GCM and fingerprints domain-separated. `NewServiceWithCustody` selects native Keychain/Secret Service, explicit age headless custody or an operator-supplied override; an external DB cannot silently initialize a missing key. | The same encrypted data and fingerprint identity must survive process restarts; a locked vault fails closed. A legacy import requires opt-in and leaves its source untouched. See [custody and recovery](key-custody.md). |
| Model policy | The model-policy engine is planned for M2/M5; **no alternative policy implementation is wired in M1**. The capability set passed to future policy must be computed from models served by eligible provider/account pairs, never an unchecked catalog list. | A chosen model must have an eligible account; an unsatisfiable choice returns a declared no-candidate decision or advances to another model, never bypasses account policy. |

## Execution order

```text
client → gateway.Handler → compiled snapshot / model policy
       → SelectionIterator (eligible account & model)
       → provideradapter.Registry (capability-validated executor)
       → upstream
```

Selection and execution are distinct. `gateway.Handler` holds a
`SelectionFactory` configured before startup and uses it in standard requests,
auto-model decisions, and WebSocket turns. On an implementation that returns nil,
the gateway fails closed: it does not call the upstream.

## How a provider is added or replaced

1. Add or update the channel descriptor in `internal/channel` for actual protocol,
   models, and route modes (no magic new provider inferred from a URL).
2. Implement `execution.Executor` **and**
   `provideradapter.RouteCapabilityValidator` in an isolated adapter package.
   Do not let the adapter select another account or mutate `AttemptSpec`.
3. Bind the provider kind in `internal/container/newProviderAdapterRegistry`.
   The `provideradapter.Registry` refuses missing/duplicate/unsupported bindings.
4. Run registry conformance plus one end-to-end request/stream; test replay policy,
   permanent denial vs cooldown and secret redaction. Add/update a `VENDORING.md`
   row with source paths, license and notices if code is imported.
5. Release an artifact. Roll back via the previous artifact; account IDs and
   encrypted state stay stable.

**Outbound policy:** automatic catalog sync is disabled by default; only
`MODELS_DEV_AUTO_SYNC_ENABLED=true` allows startup, periodic and group-change
sync. Provider executors need network for requests, and an operator may invoke
catalog synchronization. The core has **no GitHub release checker**; installer
frontends own authenticated update discovery and artifact verification. Offline
smoke must prove no automatic egress while idle.
