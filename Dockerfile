FROM --platform=$BUILDPLATFORM node:24.18.0-alpine3.24@sha256:a0b9bf06e4e6193cf7a0f58816cc935ff8c2a908f81e6f1a95432d679c54fbfd AS web-builder

WORKDIR /build
RUN corepack enable \
    && corepack install --global pnpm@11.17.0

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./web/
RUN pnpm --dir web install --frozen-lockfile

COPY internal/webui/page_routes.json internal/webui/modern_page_routes.json ./internal/webui/
COPY web ./web
RUN pnpm --dir web run build


FROM --platform=$BUILDPLATFORM golang:1.27.0-alpine3.24@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS go-builder

ARG VERSION=2.0.0-dev
ARG TARGETOS
ARG TARGETARCH
# GOPROXY uses a pipe separator: any error, including a network error, falls back to the next source.
# The default comma separator falls back only on 404/410; a transient module-proxy failure would halt the build.
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOPROXY="https://proxy.golang.org|direct"

WORKDIR /build

COPY go.mod go.sum ./
COPY third_party/cpaembedded/go.mod third_party/cpaembedded/go.sum ./third_party/cpaembedded/
RUN go mod download

COPY *.go ./
COPY internal ./internal
COPY third_party/cpaembedded ./third_party/cpaembedded
COPY --from=web-builder /build/internal/webui/dist ./internal/webui/dist
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags "-s -w -X gpt-load/internal/platform/version.Version=${VERSION}" \
    -o gpt-load


# runtime is the sole runtime definition shared by the default self-contained source build and the release prebuilt target, which reuses build-binaries output.
# Only the binary source differs; all other runtime configuration is defined here to prevent drift between the two packaging paths.
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS runtime

WORKDIR /app
RUN apk add --no-cache libcrypto3=3.5.8-r0 libssl3=3.5.8-r0 \
    && apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates \
    && addgroup -S -g 10001 demerzel \
    && adduser -S -D -H -u 10001 -G demerzel demerzel \
    && mkdir -p /app/data \
    && chown 10001:10001 /app/data \
    && chmod 0700 /app/data

ENV HOST=0.0.0.0
ENV DATA_DIR=/app/data
COPY LICENSE THIRD_PARTY_NOTICES.md /app/licenses/
COPY LICENSES/Apache-2.0.txt /app/licenses/Apache-2.0.txt
COPY LICENSES/BSD-2-Clause-dbus.txt /app/licenses/BSD-2-Clause-dbus.txt
COPY LICENSES/BSD-3-Clause-age.txt /app/licenses/BSD-3-Clause-age.txt
COPY LICENSES/Inno-Setup.txt /app/licenses/Inno-Setup.txt
COPY LICENSES/MIT.txt /app/licenses/MIT.txt
COPY LICENSES/MPL-2.0.txt /app/licenses/MPL-2.0.txt
LABEL org.opencontainers.image.source="https://github.com/a-mad-av8r/demerzel"

EXPOSE 3001 1455 54545 51121
USER 10001:10001
ENTRYPOINT ["/app/gpt-load"]


# Release path: package the cross-compiled binary produced by build-binaries directly, without recompiling it in the image.
# The release prebuilt target packages a Demerzel-owned Linux release asset.
FROM runtime AS prebuilt

ARG TARGETARCH
ARG DEMERZEL_TARGETARCH=${TARGETARCH}
# GitHub Actions artifacts may lose the executable bit; restore it explicitly.
COPY --chmod=0755 release/demerzel-linux-${DEMERZEL_TARGETARCH} /app/gpt-load


# Default target: a self-contained source build for local `docker build .` and user-built images.
# It must remain last, otherwise the default build resolves to prebuilt and requires a prebuilt artefact.
FROM runtime AS source-build

COPY --from=go-builder /build/gpt-load .
