#!/usr/bin/env bash
set -euo pipefail
umask 077

old_artifacts="${1:?usage: local-smoke.sh OLD_ARTIFACT_DIR NEW_ARTIFACT_DIR}"
new_artifacts="${2:?usage: local-smoke.sh OLD_ARTIFACT_DIR NEW_ARTIFACT_DIR}"
[[ -f "${old_artifacts}/install.sh" && -f "${new_artifacts}/install.sh" ]] || {
  printf 'both generated artifact directories must contain install.sh\n' >&2
  exit 1
}

smoke_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
trusted_verifier="${smoke_dir}/verify-release.sh"
[[ -f "${trusted_verifier}" && ! -L "${trusted_verifier}" ]] || {
  printf 'the trusted release verifier must accompany this smoke tool\n' >&2
  exit 1
}
old_version="$(bash "${trusted_verifier}" "${old_artifacts}" --print-version)"
new_version="$(bash "${trusted_verifier}" "${new_artifacts}" --print-version)"
for artifacts in "${old_artifacts}" "${new_artifacts}"; do
  bash "${trusted_verifier}" "${artifacts}" install.sh >/dev/null
  bash "${trusted_verifier}" "${artifacts}" verify-release.sh >/dev/null
done
[[ "${old_version}" != "${new_version}" ]] || {
  printf 'upgrade smoke requires two different signed versions\n' >&2
  exit 1
}

scratch="$(mktemp -d)"
trap 'rm -rf "${scratch}"' EXIT
export HOME="${scratch}/home"
mkdir -p "${HOME}"
prefix="${scratch}/homebrew/Cellar/demerzel/${old_version}"
data_dir="${HOME}/.demerzel"
config_dir="${HOME}/.config/demerzel"

bash "${old_artifacts}/install.sh" install \
  --artifacts "${old_artifacts}" \
  --prefix "${prefix}" \
  --data-dir "${data_dir}" \
  --config-dir "${config_dir}"
"${prefix}/bin/demerzel" help >/dev/null
printf 'preserve-this-database-state\n' >"${data_dir}/installer-smoke-sentinel"

bash "${new_artifacts}/install.sh" install \
  --artifacts "${new_artifacts}" \
  --prefix "${prefix}" \
  --data-dir "${data_dir}" \
  --config-dir "${config_dir}"
[[ -f "${data_dir}/installer-smoke-sentinel" ]] || { printf 'upgrade removed persistent runtime data\n' >&2; exit 1; }
"${prefix}/bin/demerzel" help >/dev/null
[[ "$(readlink "${prefix}/current")" == "versions/${new_version}" ]] || { printf 'upgrade did not activate v%s\n' "${new_version}" >&2; exit 1; }

bash "${new_artifacts}/install.sh" rollback --prefix "${prefix}"
[[ "$(readlink "${prefix}/current")" == "versions/${old_version}" ]] || { printf 'rollback did not restore v%s\n' "${old_version}" >&2; exit 1; }
[[ -f "${data_dir}/installer-smoke-sentinel" ]] || { printf 'rollback removed persistent runtime data\n' >&2; exit 1; }
"${prefix}/bin/demerzel" help >/dev/null

bash "${new_artifacts}/install.sh" uninstall --prefix "${prefix}" --data-dir "${data_dir}"
[[ ! -e "${prefix}" && -f "${data_dir}/installer-smoke-sentinel" ]] || {
  printf 'default uninstall failed to remove binaries or did not preserve runtime data\n' >&2
  exit 1
}
printf 'local artifact smoke passed: v%s -> v%s -> rollback v%s; data preserved on uninstall\n' \
  "${old_version}" "${new_version}" "${old_version}"
