#!/usr/bin/env bash
# Shared helpers for the isolated InfluxDB test environment.
#
# SECRET HANDLING RULES (enforced by convention and review, not by magic):
#   - Token/password values live only in ./secrets (Git-ignored) and are read
#     into files under ./tmp (Git-ignored) for curl `-H @file`, or into the
#     process environment for the influxdb3 CLI.
#   - Never echo a secret, never pass one as a command-line argument (argv is
#     world-readable via ps), and never write one into logs/artifacts/reports.
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

COMPOSE_FILE="${ROOT_DIR}/compose.yaml"
ENV_FILE="${ROOT_DIR}/.env"
SECRETS_DIR="${ROOT_DIR}/secrets"
TMP_DIR="${ROOT_DIR}/tmp"
LOGS_DIR="${ROOT_DIR}/logs"
ARTIFACTS_DIR="${ROOT_DIR}/artifacts"
REPORTS_DIR="${ROOT_DIR}/reports"
SEED_DIR="${ROOT_DIR}/seed"

# Load non-secret configuration. The scripts export HOST_UID/HOST_GID so
# compose runs the containers as the invoking user.
load_env() {
  if [ -f "${ENV_FILE}" ]; then
    set -a
    # shellcheck disable=SC1090
    . "${ENV_FILE}"
    set +a
  fi
  : "${INFLUXDB2_PORT:=18086}"
  : "${INFLUXDB3_PORT:=18181}"
  : "${INFLUXDB2_ORG:=ttk-test-org}"
  : "${INFLUXDB2_BUCKET:=ttk-test-bucket}"
  : "${INFLUXDB3_NODE_ID:=ttk-test-node}"
  export HOST_UID="${HOST_UID:-$(id -u)}"
  export HOST_GID="${HOST_GID:-$(id -g)}"
  export INFLUXDB2_PORT INFLUXDB3_PORT INFLUXDB2_ORG INFLUXDB2_BUCKET INFLUXDB3_NODE_ID
  export INFLUXDB2_URL="http://127.0.0.1:${INFLUXDB2_PORT}"
  export INFLUXDB3_URL="http://127.0.0.1:${INFLUXDB3_PORT}"
}

# Compose wrapper. Uses plain `docker compose`; set DOCKER_SUDO=1 to prefix sudo
# when the invoking user is not in the docker group. HOST_UID/HOST_GID are
# passed explicitly because `sudo` does not inherit the caller's environment.
compose() {
  if [ "${DOCKER_SUDO:-0}" = "1" ]; then
    sudo HOST_UID="${HOST_UID}" HOST_GID="${HOST_GID}" \
      docker compose --project-directory "${ROOT_DIR}" -f "${COMPOSE_FILE}" "$@"
  else
    docker compose --project-directory "${ROOT_DIR}" -f "${COMPOSE_FILE}" "$@"
  fi
}

docker_cmd() {
  if [ "${DOCKER_SUDO:-0}" = "1" ]; then
    sudo docker "$@"
  else
    docker "$@"
  fi
}

require_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "BLOCKED: docker CLI not found." >&2
    exit 2
  fi
  if ! docker_cmd info >/dev/null 2>&1; then
    echo "BLOCKED: cannot talk to the Docker daemon." >&2
    echo "         Start it (e.g. 'sudo dockerd > /tmp/docker.log 2>&1 &')" >&2
    echo "         or set DOCKER_SUDO=1 if your user is not in the docker group." >&2
    exit 2
  fi
}

# Prepare a curl header file for a token without ever putting it in argv.
# usage: write_auth_header <secrets-file> <header-file> <scheme>
write_auth_header() {
  local secrets_file="$1" header_file="$2" scheme="$3"
  mkdir -p "${TMP_DIR}"
  chmod 700 "${TMP_DIR}" 2>/dev/null || true
  if [ ! -r "${secrets_file}" ]; then
    echo "BLOCKED: secret file ${secrets_file} is missing or unreadable." >&2
    exit 2
  fi
  ( umask 077; printf 'Authorization: %s %s\n' "${scheme}" "$(cat "${secrets_file}")" > "${header_file}" )
  chmod 600 "${header_file}"
}

# Prepare a curl header file from a token inside an InfluxDB 3 JSON token file.
# usage: write_bearer_header_from_json <json-file> <header-file>
write_bearer_header_from_json() {
  local json_file="$1" header_file="$2"
  mkdir -p "${TMP_DIR}"
  chmod 700 "${TMP_DIR}" 2>/dev/null || true
  if [ ! -r "${json_file}" ]; then
    echo "BLOCKED: token file ${json_file} is missing or unreadable." >&2
    exit 2
  fi
  local token
  token="$(sed -E 's/.*"token"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' "${json_file}")"
  ( umask 077; printf 'Authorization: Bearer %s\n' "${token}" > "${header_file}" )
  chmod 600 "${header_file}"
  unset token
}

# Wait until a container reports healthy, with bounded retries.
# usage: wait_healthy <container-name> [attempts]
wait_healthy() {
  local name="$1" attempts="${2:-48}" i status
  for i in $(seq 1 "${attempts}"); do
    status="$(docker_cmd inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${name}" 2>/dev/null || echo missing)"
    case "${status}" in
      healthy) return 0 ;;
      missing) echo "container ${name} not found" >&2; return 1 ;;
    esac
    sleep 5
  done
  echo "container ${name} did not become healthy (last status: ${status})" >&2
  return 1
}

sanitize_stream() {
  # Redact anything that looks like a token so logs/artifacts stay secret-free.
  sed -E 's/(apiv3_)[A-Za-z0-9_-]+/\1[redacted]/g; s/(Token|Bearer) [A-Za-z0-9_.=-]{8,}/\1 [redacted]/g'
}

log_note() {
  printf '%s\n' "$*" | tee -a "${LOGS_DIR}/runner.log" >&2
}
