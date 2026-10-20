#!/usr/bin/env bash
# Tear down the isolated test stack. Scoped to this Compose project only:
# it never touches unrelated containers, networks or volumes.
#
# usage: teardown.sh            stop and remove containers + network (keep data)
#        teardown.sh --volumes  also delete the bind-mounted data dirs
#        teardown.sh --secrets  also delete generated credentials
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env
require_docker

purge_volumes=0
purge_secrets=0
for arg in "$@"; do
  case "${arg}" in
    --volumes) purge_volumes=1 ;;
    --secrets) purge_secrets=1 ;;
    *) echo "unknown option: ${arg}" >&2; exit 2 ;;
  esac
done

echo "Stopping and removing containers for project telegraf-toolkit-influxdb-test..."
compose down --remove-orphans

purge_dir() {
  # Bind-mounted data may be owned by a different container uid; remove it with
  # the same privilege used to talk to Docker so cleanup does not half-fail.
  local dir="$1"
  [ -e "${dir}" ] || return 0
  if [ "${DOCKER_SUDO:-0}" = "1" ]; then
    sudo rm -rf "${dir}"
  else
    rm -rf "${dir}"
  fi
}

if [ "${purge_volumes}" = "1" ]; then
  echo "Removing bind-mounted data directories (test data only)..."
  purge_dir "${ROOT_DIR}/data/influxdb2"
  purge_dir "${ROOT_DIR}/data/influxdb2-config"
  purge_dir "${ROOT_DIR}/data/influxdb3"
  purge_dir "${ROOT_DIR}/data/influxdb3-plugins"
fi

if [ "${purge_secrets}" = "1" ]; then
  echo "Removing generated test credentials..."
  rm -f "${SECRETS_DIR}"/influxdb2-admin-username \
        "${SECRETS_DIR}"/influxdb2-admin-password \
        "${SECRETS_DIR}"/influxdb2-admin-token \
        "${SECRETS_DIR}"/influxdb3-admin-token.json
fi

rm -f "${TMP_DIR}"/hdr-influxdb2 "${TMP_DIR}"/hdr-influxdb3
echo "Teardown complete. Unrelated containers/volumes were not touched."
