#!/usr/bin/env bash
set -euo pipefail

release_dir="${1:?release asset directory is required}"
repository="${2:-${GITHUB_REPOSITORY:-}}"
tag="${3:-${GITHUB_REF_NAME:-}}"

[[ -n "${repository}" ]] || { printf 'repository is required\n' >&2; exit 2; }
[[ -n "${tag}" ]] || { printf 'release tag is required\n' >&2; exit 2; }
[[ "${tag}" =~ ^v2\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]] || {
  printf 'invalid Demerzel 2.x release tag\n' >&2
  exit 1
}

assets=()
while IFS= read -r name; do
  case "${name}" in
    demerzel-linux-amd64|demerzel-linux-arm64|demerzel-macos-amd64|demerzel-macos-arm64|demerzel.rb|install.sh|local-smoke.sh|verify-release.sh)
      [[ -f "${release_dir}/${name}" ]] || {
        printf 'signed release asset is missing: %s\n' "${name}" >&2
        exit 1
      }
      digest="$(sha256sum "${release_dir}/${name}" | cut -d ' ' -f 1)"
      assets+=("$(jq -cn --arg name "${name}" --arg sha256 "${digest}" '{name: $name, sha256: $sha256}')")
      ;;
  esac
done <.github/release-assets.txt

(( ${#assets[@]} == 8 )) || {
  printf 'expected four native binaries, the Homebrew formula, and three installer tools in signed manifest, found %s\n' "${#assets[@]}" >&2
  exit 1
}
assets_json="$(printf '%s\n' "${assets[@]}" | jq -s 'sort_by(.name)')"
jq -n \
  --arg repository "${repository}" \
  --arg tag "${tag}" \
  --arg version "${tag#v}" \
  --argjson assets "${assets_json}" \
  '{schemaVersion: 1, repository: $repository, tag: $tag, version: $version, assets: $assets}' \
  >"${release_dir}/manifest.json.tmp"
mv "${release_dir}/manifest.json.tmp" "${release_dir}/manifest.json"
