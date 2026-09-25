#!/usr/bin/env bash
set -euo pipefail

# Assert that the org.opencontainers.image.revision config label for both linux/amd64 and linux/arm64 images equals the expected commit SHA.
#
# The registry-provided label value is untrusted input and may only be used inside the jq comparison. Only trusted ${expected} or the literal mismatch enters the shell; the observed value is written directly to stderr.
#
# Exit statuses let callers distinguish an actual mismatch from an incomplete check:
#   0 = both architectures match; stdout prints the revision
#   1 = the image is readable but the revision does not match; stderr prints the observed label
#   2 = the image manifest cannot be read (network, authentication, or a missing image); stderr prints the original error

image="${1:?image is required}"
expected="${2:?expected revision is required}"

if ! inspection="$(
  docker buildx imagetools inspect "${image}" --format '{{json .}}' 2>&1
)"; then
  printf '%s\n' "${inspection}" >&2
  exit 2
fi

# release-image-revision-validation:start
revision=mismatch
if jq -e \
  --arg expected "${expected}" \
  '[
    .image["linux/amd64"].config.Labels[
      "org.opencontainers.image.revision"
    ],
    .image["linux/arm64"].config.Labels[
      "org.opencontainers.image.revision"
    ]
  ] | all(type == "string" and . == $expected)' \
  <<<"${inspection}" >/dev/null 2>&1; then
  revision="${expected}"
fi
# release-image-revision-validation:end

if [[ "${revision}" == "mismatch" ]]; then
  printf 'image revision mismatch for %s (expected %s), actual labels: ' \
    "${image}" "${expected}" >&2
  jq -c '[
    .image["linux/amd64"].config.Labels[
      "org.opencontainers.image.revision"
    ],
    .image["linux/arm64"].config.Labels[
      "org.opencontainers.image.revision"
    ]
  ]' <<<"${inspection}" >&2 2>/dev/null || printf 'unreadable\n' >&2
  exit 1
fi

printf '%s' "${revision}"
