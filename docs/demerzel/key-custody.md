# Master-key custody and recovery (M1)

Demerzel encrypts stored credentials with AES-256-GCM and derives a separate
fingerprint key from the same master. **Losing the master key makes existing
credentials unreadable.** The plaintext `DATA_DIR/encryption.key` from the donor
is never created on a fresh boot. `AUTH_KEY` is a separate management credential:
if unset, it still lives in `DATA_DIR/auth.key` and must be protected and backed
up with the database.

## Choose one custody mode

| Mode | Configuration | Operational requirement |
|---|---|---|
| macOS desktop (default) | Leave `ENCRYPTION_KEY` and both age variables unset. | Accessible, unlocked user Keychain. The item is non-synchronizable and available only while unlocked. Its account is derived from the **canonical absolute `DATA_DIR` path**; relocating the directory without migrating the vault key will fail closed. Back up/restore the appropriate Keychain with the database. |
| Linux desktop (default) | Leave `ENCRYPTION_KEY` and both age variables unset. | Unlocked default Secret Service collection over the user's session D-Bus (for example GNOME Keyring/KWallet). Its item is scoped to the canonical `DATA_DIR` path. A missing service or locked collection is an error, never permission to create a local plaintext key. Back up/restore the provider's vault with the database. |
| Headless / portable | Set **both** `DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE` and `DEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT`. | The external identity must be an owner-only regular file, outside `DATA_DIR`; its X25519 identity must match the recipient. Demerzel stores only `DATA_DIR/encryption.key.age` (0600), never the identity there. Back up the identity **separately** from the encrypted data/database; verify an actual restore. |
| Explicit override (CI / operator recovery) | Set `ENCRYPTION_KEY` to the original key material. | The key is in the process environment; control environment-file ACLs and logs. This takes precedence over native custody; do not set age variables with it. The key is not written into a vault by this mode. |

**macOS service-mode limit (observed on an unsigned rebuild):** the previously
created Keychain item was readable on restart with the same binary. After
rebuilding the binary, a second startup blocked inside
`Security.framework/SecItemCopyMatching` before the HTTP listener opened. The
binary was ad-hoc/linker-signed with no stable designated requirement; a GUI
Keychain access-control approval is the likely cause, not a missing database
key. Do not rely on this interactive mode for unattended `launchd` startups or
upgrades. **Provision explicit age custody from the first boot** for headless
or background service use; Demerzel never silently switches an existing native
key to age. Stable Developer-ID signing and a real upgrade/ACL test are release
gates before Keychain becomes a supported unattended service backend. Do not
solve a blocked lookup by generating a different key against the existing DB.

Example provisioning of headless custody (with the [age](https://github.com/FiloSottile/age)
CLI installed separately):

```sh
umask 077
mkdir -p "$HOME/.config/demerzel"
age-keygen -o "$HOME/.config/demerzel/identity.txt"
# Copy the *public* recipient into your environment; never paste the private key.
age-keygen -y "$HOME/.config/demerzel/identity.txt"
export DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE="$HOME/.config/demerzel/identity.txt"
export DEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT='age1...replace-with-output-above'
```

Use the actual recipient printed by `age-keygen -y`; `age1...` above is **not**
a working recipient. The executable links the age library; it does not execute
`age-keygen` at runtime. Never put the private identity inside `DATA_DIR`, in
shell history, or in the database backup.

A fresh managed SQLite installation may generate the first key when its selected
vault is available. An **external `DATABASE_DSN` never generates a new key by
default**: if the vault is missing, it refuses startup rather than silently
making existing encrypted data unreadable. Only after independently confirming
the external database is empty, set
`DEMERZEL_ENCRYPTION_KEY_INITIALIZE_EXTERNAL=1` for its first startup, then
unset it; subsequent restarts load the existing key. An existing managed
`DATA_DIR/gpt-load.db` (or SQLite recovery sidecar) likewise blocks generation
if the custody item is missing. `ENCRYPTION_KEY` and guarded legacy import are
explicit recovery options, not automatic fallback behavior.

At startup, Demerzel stores a non-secret, domain-separated key-identity HMAC in
the database under `_internal.encryption.master_key_identity.v1`. Every later
boot checks it **before** loading runtime credentials or accepting traffic. For
older databases without a marker, an existing encrypted access key, account or
proxy must authenticate under the selected key before the marker is created.
A mismatched key fails closed; neither an existing ciphertext nor its marker
is rotated to make startup succeed.

### Find a native vault item for supervised escrow

The Keychain account / Linux Secret Service `installation` attribute is the
non-secret digest of the **canonical** absolute `DATA_DIR` path. The installed
binary prints that locator without opening or revealing the vault:

```sh
export DATA_DIR="$HOME/.demerzel"
locator="$(demerzel key-locator --data-dir "$DATA_DIR")"
printf 'Vault account locator: %s\n' "$locator"
```

For a supervised escrow export, set `ESCROW_FILE` to a destination protected
independently from the database backup (for example, a mounted encrypted
off-machine vault). The export is interactive, never part of the service:

```sh
: "${ESCROW_FILE:?Set a protected off-machine escrow file path first}"
umask 077
# macOS, with login Keychain unlocked:
security find-generic-password -s io.demerzel.encryption.master-key.v1 -a "$locator" -w > "$ESCROW_FILE"
# Linux instead, with the Secret Service collection unlocked and secret-tool installed:
# secret-tool lookup application io.demerzel purpose encryption.master-key.v1 installation "$locator" > "$ESCROW_FILE"
```

Both commands reveal the 64-hex master key: **never paste their output into
chat, logs, shell history, or a database backup.** Keep the escrow copy
separately custodied and prove a restore before retiring any source. If
restoring to a different absolute path, the locator changes; a supervised
`DEMERZEL_ENCRYPTION_KEY_IMPORT_LEGACY=1` import from a 0600 source file can
bind the original master to that new installation, but do not remove the
temporary source until restore proof and backup gate approval.

## Import donor plaintext key without rotating it

1. Stop the old process. Record a consistent database snapshot **and a separate
   protected copy** of `DATA_DIR/encryption.key`, `DATA_DIR/auth.key`, and the
   relevant config. For a live SQLite database, stop writes before copying its
   main file and sidecars; external DBs need their own consistent snapshot.
2. Configure the native vault or headless age mode. Ensure `ENCRYPTION_KEY` is
   unset. Set `DEMERZEL_ENCRYPTION_KEY_IMPORT_LEGACY=1` and start once. Demerzel
   checks that the legacy file is owner-only and well formed, stores the **same**
   master in custody, reads it back, and refuses a mismatched existing item.
3. Verify that an existing credential still decrypts on a real request without
   printing it. Unset the import flag; restart the process and repeat that
   verification. Keep the original `encryption.key` unchanged during these
   checks; its presence also makes a mismatched new vault item fail closed.
4. Do **not** delete the legacy source or retire its backup merely because the
   import returned success. Restore the database **and** key into a separate
   recovery environment and prove a real credential decrypt before replacing
   any backup path. Treat deletion/rotation as a separate gated operation.

## Backup / restore boundary

Irreplaceable state is the database, the master-key custody item or age identity
and encrypted key file, `auth.key` when generated, and operator configuration.
`runtime-state.checkpoint.json` contains scheduling and credential-health state;
copy it when present, or explicitly accept resetting that state. Catalog cache
and logs are derived/retention-managed; do not mistake them for key backups.
A copy of the database alone **cannot** recover encrypted accounts.
For headless recovery, restore the stopped DB, `DATA_DIR/encryption.key.age`,
`auth.key`, and runtime checkpoint into an isolated DATA_DIR; restore the
separately secured identity outside it, use the same recipient, and verify a
real credential decrypt before cutting traffic over. Native-vault recovery
also needs the original OS vault item: a copied `DATA_DIR` at a new canonical
path will not find it. Never generate a replacement key against an existing DB.

M1 local restore drill (2026-09-24): a stopped SQLite database, auth key, age
ciphertext and runtime checkpoint were copied to a **different** DATA_DIR; the
separately retained age identity decrypted the persisted provider credential,
and the original access key completed a request against a local fake provider.
This tests the file/identity pairing, not an off-device scheduled backup lane.

No scheduled off-device producer, restore-proof, retention policy, or monitoring
is configured by this M1 code. **It is not a backup system or a production
release gate pass.** The estate owner must provision and restore-test those
lanes before claiming a deployment is protected.
