# Account CLI

`demerzel accounts` manages API-key credentials through Demerzel's authenticated
loopback control API. It does not access the database directly. Choose an
existing API-key group and pass both its group ID and, for mutations, the
credential ID returned by the API.

```sh
demerzel accounts list
demerzel accounts list --group 7
demerzel accounts add --group 7 --stdin < account-key.txt
demerzel accounts add --group 7 --key-file "$HOME/.config/private/account-key"
demerzel accounts label --group 7 --credential 41 --label "Production account"
demerzel accounts disable --group 7 --credential 41
demerzel accounts restore --group 7 --credential 41
demerzel accounts remove --group 7 --credential 41
```

`list` reports the configured label, account identity or masked credential,
configured/effective status, quota observations when the server has them,
cooldown expiry, and health/authentication observations. API-key accounts
without upstream quota observations show `not reported`. The table never
returns or prints the original credential.

`add` reads exactly one API key from stdin or a regular file. The key is never
accepted as a command-line argument. A key file must be owner-only (for example
mode `0600` or `0400`) and must not be a symlink. Stdin is useful for pipes or
secret managers that can write directly to the command's input. Do not paste a
key into a shell command or shell history.

`label` updates the credential label. Pass `--label ""` to clear it. A label
update is reported as successful only if the control API returns the requested
label. `disable` and `restore` require an explicit group and credential ID and
use the batch API with that one ID; `restore` re-enables a manually disabled
credential. `remove` permanently deletes the explicitly identified credential.

## Control API and credentials

The CLI uses these authenticated API contracts:

- `GET /api/modern/groups` and paginated
  `GET /api/modern/groups/:group_id/credentials` for account display facts.
- `POST /api/groups/:group_id/credentials/import` to import one API key, with a
  fresh `Idempotency-Key` on the request.
- `PUT /api/groups/:group_id/credentials/:credential_id` with `{"label":"…"}`
  to change a label; an empty string clears it. The returned item must contain
  the same credential ID and label before the CLI reports success.
- `POST /api/groups/:group_id/credentials/batch` with an explicit one-element
  `credential_ids` list and the `disable` or `enable` action.
- `DELETE /api/groups/:group_id/credentials/:credential_id` to remove one
  credential.

The `label` field on credential list/update responses is part of the server API
contract. The CLI sends management requests only to an HTTP(S) loopback origin;
redirects and non-loopback URLs are rejected so the bearer key is not forwarded
to another host. API error bodies are not printed.

## Management key and data root

The management key comes from the `AUTH_KEY` environment variable when it is
non-empty, or from an owner-only key file. The default file is
`$DATA_DIR/auth.key`. Use `--data-dir` or `--auth-key-file` to select the same
root/key file used by the running server. If `DATA_DIR` is not set, the CLI
uses `$HOME/.demerzel` on macOS or `${XDG_DATA_HOME:-$HOME/.local/share}/demerzel`
on Linux (`XDG_DATA_HOME` must be absolute); other platforms use their
per-user config root under `demerzel/data`. The CLI does not create data
directories or fall back to a repository-relative path. The control URL defaults
to `http://127.0.0.1:3001`; `--url` or `DEMERZEL_CONTROL_URL` may select another
loopback port.

The CLI never prints the management key, upstream key, request body, or raw
control API error body. Commands return a nonzero exit code for invalid input,
unsafe paths/URLs, authentication failures, API errors, and unconfirmed
mutations.
