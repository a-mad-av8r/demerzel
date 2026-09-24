#!/usr/bin/env bash
set -euo pipefail

artifact_dir="${1:?usage: verify-release.sh ARTIFACT_DIR [ASSET_NAME] [--print-version]}"
asset="${2:-}"
print_version=false
if [[ "${asset}" == "--print-version" ]]; then
  asset=
  print_version=true
elif [[ "${3:-}" == "--print-version" ]]; then
  print_version=true
fi

if [[ -z "${asset}" ]]; then
  case "$(uname -s)" in
    Darwin) os=macos ;;
    Linux) os=linux ;;
    *) printf 'unsupported native installer platform: %s\n' "$(uname -s)" >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) printf 'unsupported native installer architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
  esac
  asset="demerzel-${os}-${arch}"
fi

manifest="${artifact_dir}/manifest.json"
bundle="${artifact_dir}/manifest.sigstore.json"
[[ -f "${manifest}" && -f "${bundle}" ]] || {
  printf 'signed manifest or Sigstore bundle is missing from %s\n' "${artifact_dir}" >&2
  exit 1
}
command -v jq >/dev/null 2>&1 || { printf 'jq is required to verify release assets\n' >&2; exit 127; }
cosign_bin="${COSIGN_BIN:-cosign}"
command -v "${cosign_bin}" >/dev/null 2>&1 || { printf 'cosign is required to verify release signatures\n' >&2; exit 127; }

identity='^https://github\.com/a-mad-av8r/demerzel/\.github/workflows/release\.yml@refs/tags/v2\.[0-9]+\.[0-9]+(-[A-Za-z0-9][A-Za-z0-9.-]*)?$'
"${cosign_bin}" verify-blob \
  --bundle "${bundle}" \
  --certificate-identity-regexp "${identity}" \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  "${manifest}"

jq -e \
  --arg repository 'a-mad-av8r/demerzel' \
  --arg asset "${asset}" \
  '(.schemaVersion == 1) and
   (.repository == $repository) and
   (.tag | type == "string" and test("^v2\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\\.[0-9A-Za-z-]+)*)?$")) and
   (.version == (.tag | ltrimstr("v"))) and
   (.assets | type == "array") and
   ([(.assets[] | select(.name == $asset and (.sha256 | test("^[a-f0-9]{64}$"))))] | length == 1)' \
  "${manifest}" >/dev/null || {
  printf 'manifest has an invalid Demerzel release identity or asset entry\n' >&2
  exit 1
}

asset_path="${artifact_dir}/${asset}"
[[ -f "${asset_path}" ]] || { printf 'release asset is missing: %s\n' "${asset}" >&2; exit 1; }
expected="$(jq -er --arg asset "${asset}" '.assets[] | select(.name == $asset) | .sha256' "${manifest}")"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${asset_path}" | cut -d ' ' -f 1)"
else
  actual="$(shasum -a 256 "${asset_path}" | cut -d ' ' -f 1)"
fi
[[ "${actual}" == "${expected}" ]] || {
  printf 'signed manifest digest mismatch for %s\n' "${asset}" >&2
  exit 1
}

version="$(jq -er '.version' "${manifest}")"
if [[ "${print_version}" == true ]]; then
  printf '%s\n' "${version}"
else
  printf 'verified Demerzel v%s asset %s\n' "${version}" "${asset}"
fi
