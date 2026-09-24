# Rootless Podman installation

`docker-compose.yml` builds the local Demerzel image by default; it never pulls
or deploys a donor image. The application runs as UID/GID `10001`, binds HTTP
and OAuth callback ports to loopback by default, keeps `/app/data` in the
Compose-owned `demerzel-data` named volume, and mounts the explicit age identity
read-only at `/run/secrets/demerzel-age-identity` outside that volume.

## Provision external age custody

Use an isolated rootless Podman host or Podman Machine. Create the identity on
the host, not in a container or volume:

```sh
umask 077
mkdir -p "$HOME/.config/demerzel"
chmod 700 "$HOME/.config/demerzel"
age-keygen -o "$HOME/.config/demerzel/identity.txt"
recipient="$(age-keygen -y "$HOME/.config/demerzel/identity.txt")"
```

Copy `.env.example` to `.env`, restrict it to the current user, and set these
two values to the absolute identity path and the public recipient printed by
`age-keygen -y`:

```sh
chmod 600 .env
```

The private identity stays in `$HOME/.config/demerzel/identity.txt`; `.env`
contains only its path and public recipient. Do not set `ENCRYPTION_KEY` for
this deployment. If a database already exists but its external identity is
missing, restore the original identity and matching data; never create a new key
against existing state.

## Build and run

The regular local build target compiles the web and Go stages and can consume
significant disk. Use it only on a dedicated host with adequate free space.
`podman-compose` needs rootless keep-id mapping so the owner-only host identity
appears as container UID 10001; `make container-up` sets this automatically.

```sh
PODMAN_USERNS=keep-id:uid=10001,gid=10001 podman-compose up --build -d
```

For a generated native release binary, select the small prebuilt runtime stage
instead. The checkout supplies the Dockerfile and license files; the signed
artifact supplies the already-built Linux binary. Example for an arm64 Podman
VM:

```sh
mkdir -p release
cp /path/to/release-assets/demerzel-linux-arm64 release/demerzel-linux-arm64
DEMERZEL_BUILD_TARGET=prebuilt DEMERZEL_TARGETARCH=arm64 \
  PODMAN_USERNS=keep-id:uid=10001,gid=10001 podman-compose up --build -d
```

Use `DEMERZEL_TARGETARCH=amd64` with the amd64 asset on an amd64 Podman host.
The prebuilt stage uses the same pinned minimal runtime as release CI and does
not run Go, pnpm, or Trivy. This is the appropriate local image smoke path when
only a host-crossbuilt release binary is available.

`docker-compose.yml` maps the rootless host user to container UID/GID 10001 so
the mounted owner-only identity remains readable to the non-root process. Keep
the Podman identity mount outside the `demerzel-data` volume. The management UI
is on `http://127.0.0.1:3001` by default.

The secret-bearing account CLI uses the owner-only Unix socket at
`/app/data/control.sock`, not HTTP. Run it in the same container/user namespace
as the gateway so both processes use the named-volume `DATA_DIR`:

```sh
podman-compose exec -T demerzel /app/gpt-load accounts list
```

The HTTP listener remains for the browser/data plane on loopback; do not use it
as a control-CLI transport.

## Persistence, backup, and removal

`podman-compose down` stops and removes containers but preserves the named
volume. It does not remove `auth.key`, the SQLite database/sidecars, encrypted
age custody, or the runtime checkpoint. Back up the stopped volume and the age
identity separately; also retain the recipient and operator configuration. A
volume alone cannot recover credentials if its external identity is lost.

Do not use `podman-compose down --volumes` for ordinary uninstall. The narrow
purge path verifies Compose ownership and requires both `--purge` and typing the
exact volume name:

```sh
make container-down
make container-purge
```

The default project volume name is `demerzel_demerzel-data`. The external age
identity remains untouched after purge. `COMPOSE_PROJECT_NAME` changes the
project-scoped volume name; the purge helper validates the matching Compose
labels before removal.

## Local Podman smoke safety

A local image smoke requires an isolated Podman host/provider with enough free
disk for the runtime image and temporary data. Do not run source builds, pull a
large scanner image, prune resources, or stop unrelated workloads to make room.
The shared `kos-e020-uat` Podman VM was measured at 97% disk usage with 3.2 GiB
free; do not use it without a separately approved maintenance window. The
separately provisioned development VM cannot run concurrently with that UAT VM.
Use another isolated provider/host or wait for an approved window. No Podman
smoke is claimed by this work.

The owned GHCR image is private and release publication is still approval-gated;
no cloud runtime or managed deployment is created here.

See the [credential accounts guide](accounts.md) and [key-custody runbook](key-custody.md)
for account operations and restore steps.
