#!/usr/bin/env bash
set -euo pipefail
umask 077

usage() {
  cat >&2 <<'EOF'
Usage:
  install.sh install --artifacts DIR [--prefix DIR] [--data-dir DIR] [--config-dir DIR]
  install.sh rollback [--prefix DIR]
  install.sh status [--prefix DIR]
  install.sh uninstall [--prefix DIR] [--data-dir DIR] [--purge]

The default install prefix is ~/.local/opt/demerzel. Uninstall always preserves
runtime data and the external age identity unless --purge is explicitly confirmed.
EOF
}

fail() { printf '%s\n' "$*" >&2; exit 1; }

home="${HOME:?HOME must be set}"
prefix="${DEMERZEL_PREFIX:-${home}/.local/opt/demerzel}"
data_dir="${DEMERZEL_DATA_DIR:-}"
config_dir="${DEMERZEL_CONFIG_DIR:-${home}/.config/demerzel}"
artifact_dir=
purge=false

platform_asset() {
  local os arch
  case "$(uname -s)" in
    Darwin) os=macos ;;
    Linux) os=linux ;;
    *) fail "unsupported installer OS: $(uname -s); native installation supports macOS and Linux only" ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) fail "unsupported installer architecture: $(uname -m)" ;;
  esac
  printf 'demerzel-%s-%s' "${os}" "${arch}"
}

default_data_dir() {
  case "$(uname -s)" in
    Darwin) printf '%s/.demerzel' "${home}" ;;
    Linux) printf '%s/demerzel' "${XDG_DATA_HOME:-${home}/.local/share}" ;;
    *) fail "unsupported installer OS: $(uname -s)" ;;
  esac
}

validate_prefix() {
  [[ "${prefix}" == /* ]] || fail "install prefix must be an absolute path: ${prefix}"
  [[ ! -L "${prefix}" ]] || fail "install prefix must not be a symlink"
  if [[ -n "${data_dir}" ]]; then
    [[ "${data_dir}" == /* ]] || fail "DATA_DIR must be an absolute path"
    [[ ! -L "${data_dir}" ]] || fail "DATA_DIR must not be a symlink"
    if [[ -d "${data_dir}" ]]; then
      data_dir="$(cd "${data_dir}" && pwd -P)"
    fi
  fi
  case "${prefix}" in
    /|"${home}"|"${home}/.demerzel"|"${home}/.local")
      fail "refusing unsafe install prefix: ${prefix}"
      ;;
  esac
  mkdir -p "${prefix}"
  prefix="$(cd "${prefix}" && pwd -P)"
  case "${prefix}" in
    /|"${home}"|"${home}/.demerzel"|"${home}/.local")
      fail "refusing unsafe install prefix: ${prefix}"
      ;;
  esac
  if [[ -n "${data_dir}" ]]; then
    case "${prefix}" in
      "${data_dir}"|"${data_dir}/"*) fail "install prefix must be outside DATA_DIR" ;;
    esac
  fi
}

current_target() {
  local target
  [[ -L "${prefix}/current" ]] || return 1
  target="$(readlink "${prefix}/current")"
  [[ "${target}" == versions/* && -d "${prefix}/${target}" ]] || fail "invalid current-version link in ${prefix}"
  printf '%s' "${target#versions/}"
}

write_launcher() {
  local recipient="$1"
  local launcher="${prefix}/bin/demerzel"
  {
    printf '#!/usr/bin/env bash\nset -euo pipefail\n'
    printf 'demerzel_data_dir_default=%q\n' "${data_dir}"
    printf 'demerzel_identity_file_default=%q\n' "${config_dir}/identity.txt"
    printf 'demerzel_recipient_default=%q\n' "${recipient}"
    printf 'export DATA_DIR="${DATA_DIR:-${demerzel_data_dir_default}}"\n'
    printf 'export DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE="${DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE:-${demerzel_identity_file_default}}"\n'
    printf 'export DEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT="${DEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT:-${demerzel_recipient_default}}"\n'
    printf 'cd %q\n' "${prefix}"
    printf 'exec %q/current/bin/gpt-load "$@"\n' "${prefix}"
  } >"${launcher}.tmp"
  chmod 0755 "${launcher}.tmp"
  mv -f "${launcher}.tmp" "${launcher}"
}

provision_age_identity() {
  local existed=false recipient identity="${config_dir}/identity.txt"
  command -v age-keygen >/dev/null 2>&1 || fail "age-keygen is required to provision first-boot custody"
  [[ ! -L "${data_dir}" ]] || fail "DATA_DIR must not be a symlink"
  [[ ! -L "${identity}" ]] || fail "age identity must be a regular file outside DATA_DIR"
  mkdir -p "${data_dir}" "${config_dir}"
  data_dir="$(cd "${data_dir}" && pwd -P)"
  config_dir="$(cd "${config_dir}" && pwd -P)"
  identity="${config_dir}/identity.txt"
  case "${identity}" in
    "${data_dir}"|"${data_dir}/"*) fail "age identity must be outside DATA_DIR" ;;
  esac
  chmod 0700 "${config_dir}"

  shopt -s nullglob dotglob
  local entries=("${data_dir}"/*)
  shopt -u nullglob dotglob
  ((${#entries[@]} == 0)) || existed=true
  if [[ ! -e "${identity}" ]]; then
    [[ "${existed}" == false ]] || fail "data already exists at ${data_dir} but the external age identity is missing; restore the original identity instead of generating a new key"
    age-keygen -o "${identity}" >/dev/null
  fi
  [[ -f "${identity}" && ! -L "${identity}" ]] || fail "age identity must be a regular file outside DATA_DIR"
  chmod 0600 "${identity}"
  recipient="$(age-keygen -y "${identity}")" || fail "cannot read the configured age identity"
  [[ "${recipient}" =~ ^age1[0-9a-z]+$ ]] || fail "configured age identity did not produce a valid recipient"
  chmod 0700 "${data_dir}"
  printf '%s' "${recipient}"
}

install_artifact() {
  local asset version old_version target temp_binary recipient
  [[ -n "${artifact_dir}" && -d "${artifact_dir}" ]] || fail "install requires --artifacts DIR"
  asset="$(platform_asset)"
  [[ -f "${artifact_dir}/install.sh" && -f "${artifact_dir}/verify-release.sh" ]] || fail "artifact directory is missing signed installer tools"
  version="$(bash "${artifact_dir}/verify-release.sh" "${artifact_dir}" "${asset}" --print-version)"
  [[ "${version}" =~ ^2\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || fail "unsupported signed release version: ${version}"
  data_dir="${data_dir:-$(default_data_dir)}"
  [[ "${data_dir}" == /* && "${config_dir}" == /* ]] || fail "DATA_DIR and age identity directory must be absolute paths"
  [[ ! -L "${data_dir}" ]] || fail "DATA_DIR must not be a symlink"
  [[ ! -L "${config_dir}" ]] || fail "age identity directory must not be a symlink"
  mkdir -p "${data_dir}" "${config_dir}"
  data_dir="$(cd "${data_dir}" && pwd -P)"
  config_dir="$(cd "${config_dir}" && pwd -P)"
  validate_prefix
  case "${config_dir}/identity.txt" in
    "${data_dir}"|"${data_dir}/"*) fail "age identity must be outside DATA_DIR" ;;
  esac
  old_version=
  if [[ -e "${prefix}/current" || -L "${prefix}/current" ]]; then
    [[ -L "${prefix}/current" ]] || fail "current version pointer is not a symlink"
    old_version="$(current_target)"
  fi
  mkdir -p "${prefix}/versions" "${prefix}/bin"
  chmod 0700 "${prefix}" "${prefix}/versions"
  target="${prefix}/versions/${version}"
  if [[ -e "${target}/bin/gpt-load" ]]; then
    local old_hash new_hash
    if command -v sha256sum >/dev/null 2>&1; then
      old_hash="$(sha256sum "${target}/bin/gpt-load" | cut -d ' ' -f 1)"
      new_hash="$(sha256sum "${artifact_dir}/${asset}" | cut -d ' ' -f 1)"
    else
      old_hash="$(shasum -a 256 "${target}/bin/gpt-load" | cut -d ' ' -f 1)"
      new_hash="$(shasum -a 256 "${artifact_dir}/${asset}" | cut -d ' ' -f 1)"
    fi
    [[ "${old_hash}" == "${new_hash}" ]] || fail "version ${version} is already installed with different bytes"
  else
    mkdir -p "${target}/bin"
    temp_binary="${target}/bin/gpt-load.tmp.$$"
    cp "${artifact_dir}/${asset}" "${temp_binary}"
    chmod 0755 "${temp_binary}"
    mv "${temp_binary}" "${target}/bin/gpt-load"
    cp "${artifact_dir}/install.sh" "${prefix}/install.sh.tmp.$$"
    chmod 0755 "${prefix}/install.sh.tmp.$$"
    mv -f "${prefix}/install.sh.tmp.$$" "${prefix}/install.sh"
  fi
  recipient="$(provision_age_identity)"
  if [[ -n "${old_version}" && "${old_version}" != "${version}" ]]; then
    ln -sfn "versions/${old_version}" "${prefix}/previous.next"
    mv -f "${prefix}/previous.next" "${prefix}/previous"
  fi
  ln -sfn "versions/${version}" "${prefix}/current.next"
  mv -f "${prefix}/current.next" "${prefix}/current"
  write_launcher "${recipient}"
  printf 'installed Demerzel v%s at %s; data remains at %s\n' "${version}" "${prefix}/current" "${data_dir}"
}

rollback_install() {
  local current previous
  data_dir="${data_dir:-$(default_data_dir)}"
  validate_prefix
  current="$(current_target)" || fail "no installed Demerzel version to roll back"
  [[ -L "${prefix}/previous" ]] || fail "no previous version is available for rollback"
  previous="$(readlink "${prefix}/previous")"
  [[ "${previous}" == versions/* && -x "${prefix}/${previous}/bin/gpt-load" ]] || fail "previous version is missing or invalid"
  ln -sfn "versions/${current}" "${prefix}/previous.next"
  mv -f "${prefix}/previous.next" "${prefix}/previous"
  ln -sfn "${previous}" "${prefix}/current.next"
  mv -f "${prefix}/current.next" "${prefix}/current"
  printf 'rolled back Demerzel to v%s\n' "${previous#versions/}"
}

uninstall_install() {
  data_dir="${data_dir:-$(default_data_dir)}"
  validate_prefix
  if [[ "${purge}" == true ]]; then
    [[ "${data_dir}" == /* ]] || fail "DATA_DIR must be an absolute path"
    [[ ! -L "${data_dir}" ]] || fail "refusing to purge a symlinked DATA_DIR"
    if [[ -d "${data_dir}" ]]; then
      data_dir="$(cd "${data_dir}" && pwd -P)"
    fi
    case "$(basename "${data_dir}")" in
      .demerzel|demerzel) ;;
      *) fail "--purge only removes an app-owned .demerzel or demerzel data directory" ;;
    esac
    [[ "${data_dir}" != / && "${data_dir}" != "${home}" && "${data_dir}" != "${config_dir}" ]] || fail "refusing unsafe --purge path: ${data_dir}"
    printf 'Type exactly PURGE %s to permanently remove only this Demerzel data directory: ' "${data_dir}" >&2
    local confirmation
    IFS= read -r confirmation || fail 'purge confirmation was not provided'
    [[ "${confirmation}" == "PURGE ${data_dir}" ]] || fail 'purge confirmation did not match; data was preserved'
  fi
  rm -rf -- "${prefix}"
  if [[ "${purge}" == true && -d "${data_dir}" ]]; then
    rm -rf -- "${data_dir}"
  fi
  printf 'removed Demerzel binaries; runtime data %s\n' "$([[ "${purge}" == true ]] && printf 'was explicitly purged' || printf 'was preserved')"
}

command_name="${1:-}"
[[ -n "${command_name}" ]] || { usage; exit 2; }
shift
while (($#)); do
  case "$1" in
    --artifacts) (($# >= 2)) || fail '--artifacts requires a directory'; artifact_dir="$2"; shift 2 ;;
    --prefix) (($# >= 2)) || fail '--prefix requires a directory'; prefix="$2"; shift 2 ;;
    --data-dir) (($# >= 2)) || fail '--data-dir requires a directory'; data_dir="$2"; shift 2 ;;
    --config-dir) (($# >= 2)) || fail '--config-dir requires a directory'; config_dir="$2"; shift 2 ;;
    --purge) purge=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown option: $1" ;;
  esac
done
[[ "${purge}" != true || "${command_name}" == "uninstall" ]] || fail "--purge is only valid with uninstall"

case "${command_name}" in
  install) install_artifact ;;
  rollback) rollback_install ;;
  status)
    current="$(current_target)" || fail "no installed Demerzel version at ${prefix}"
    printf 'Demerzel v%s (%s)\n' "${current}" "${prefix}/current"
    ;;
  uninstall) uninstall_install ;;
  *) usage; fail "unknown command: ${command_name}" ;;
esac
