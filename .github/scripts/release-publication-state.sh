#!/usr/bin/env bash
set -euo pipefail

required_inputs=(
  RELEASE_EXPECTED_SHA
  RELEASE_GITHUB_STATE
  RELEASE_GITHUB_TARGET_SHA
  RELEASE_GITHUB_ASSETS
  RELEASE_GHCR_STATE
  RELEASE_GHCR_DIGEST
  RELEASE_GHCR_REVISION
)
for input in "${required_inputs[@]}"; do
  if ! printenv "${input}" >/dev/null; then
    printf 'missing required input: %s\n' "${input}" >&2
    exit 1
  fi
done

if [[ ! "${RELEASE_EXPECTED_SHA}" =~ ^[0-9a-f]{40}$ ]]; then
  printf 'RELEASE_EXPECTED_SHA must be a 40-character commit SHA\n' >&2
  exit 1
fi

case "${RELEASE_GITHUB_STATE}" in
  absent)
    if [[ -n "${RELEASE_GITHUB_TARGET_SHA}" || "${RELEASE_GITHUB_ASSETS}" != absent ]]; then
      printf 'GitHub absent state contains present metadata\n' >&2
      exit 1
    fi
    ;;
  present)
    if [[ -z "${RELEASE_GITHUB_TARGET_SHA}" ||
      ( "${RELEASE_GITHUB_ASSETS}" != match && "${RELEASE_GITHUB_ASSETS}" != mismatch ) ]]; then
      printf 'GitHub present state has incomplete metadata\n' >&2
      exit 1
    fi
    ;;
  *)
    printf 'invalid RELEASE_GITHUB_STATE\n' >&2
    exit 1
    ;;
esac

case "${RELEASE_GHCR_STATE}" in
  absent)
    if [[ "${RELEASE_GHCR_DIGEST}" != absent || -n "${RELEASE_GHCR_REVISION}" ]]; then
      printf 'GHCR absent state contains present metadata\n' >&2
      exit 1
    fi
    ;;
  present)
    if [[ -z "${RELEASE_GHCR_DIGEST}" || "${RELEASE_GHCR_DIGEST}" == absent || -z "${RELEASE_GHCR_REVISION}" ]]; then
      printf 'GHCR present state has incomplete metadata\n' >&2
      exit 1
    fi
    ;;
  *)
    printf 'invalid RELEASE_GHCR_STATE\n' >&2
    exit 1
    ;;
esac

if [[ "${RELEASE_GITHUB_STATE}" == absent && "${RELEASE_GHCR_STATE}" == absent ]]; then
  printf 'publication_state=fresh\n'
  printf 'write_mode=publish\n'
  exit 0
fi

if [[ "${RELEASE_GITHUB_STATE}" == present ]] &&
  [[ "${RELEASE_GITHUB_TARGET_SHA}" != "${RELEASE_EXPECTED_SHA}" || "${RELEASE_GITHUB_ASSETS}" != match ]]; then
  printf 'publication_state=conflict\n'
  printf 'write_mode=blocked\n'
  exit 0
fi
if [[ "${RELEASE_GHCR_STATE}" == present && "${RELEASE_GHCR_REVISION}" != "${RELEASE_EXPECTED_SHA}" ]]; then
  printf 'publication_state=conflict\n'
  printf 'write_mode=blocked\n'
  exit 0
fi

if [[ "${RELEASE_GITHUB_STATE}" == present && "${RELEASE_GHCR_STATE}" == present ]]; then
  printf 'publication_state=consistent\n'
  printf 'write_mode=verify\n'
  exit 0
fi

printf 'publication_state=partial\n'
printf 'write_mode=publish\n'
