#!/usr/bin/env bash
set -euo pipefail

project="${COMPOSE_PROJECT_NAME:-demerzel}"
purge=false
while (($#)); do
  case "$1" in
    --purge) purge=true; shift ;;
    --project) (($# >= 2)) || { printf '%s\n' '--project requires a name' >&2; exit 2; }; project="$2"; shift 2 ;;
    -h|--help) printf 'Usage: purge-volume.sh --purge [--project NAME]\n'; exit 0 ;;
    *) printf 'unknown option: %s\n' "$1" >&2; exit 2 ;;
  esac
done
[[ "${purge}" == true ]] || { printf 'refusing to remove data without the explicit --purge gate\n' >&2; exit 2; }
[[ "${project}" =~ ^[A-Za-z0-9_-]+$ ]] || { printf 'invalid Compose project name\n' >&2; exit 2; }
command -v podman >/dev/null 2>&1 || { printf 'podman is required\n' >&2; exit 127; }

volume="${project}_demerzel-data"
labels="$(podman volume inspect "${volume}" --format '{{json .Labels}}')" || {
  printf 'Demerzel-owned volume not found: %s\n' "${volume}" >&2
  exit 1
}
command -v jq >/dev/null 2>&1 || { printf 'jq is required to verify volume ownership\n' >&2; exit 127; }
printf '%s\n' "${labels}" | jq -e \
  --arg project "${project}" \
  '."com.docker.compose.project" == $project and ."io.demerzel.data" == "true"' \
  >/dev/null || { printf 'volume labels do not identify this Compose-owned Demerzel data volume\n' >&2; exit 1; }

printf 'Type exactly PURGE %s to permanently delete this Demerzel data volume: ' "${volume}" >&2
IFS= read -r confirmation || { printf 'purge confirmation was not provided\n' >&2; exit 1; }
[[ "${confirmation}" == "PURGE ${volume}" ]] || { printf 'confirmation did not match; volume was preserved\n' >&2; exit 1; }
podman volume rm "${volume}"
