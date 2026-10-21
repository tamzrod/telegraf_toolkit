#!/usr/bin/env bash
# Generate throwaway test credentials for the isolated InfluxDB stack.
#
# These are NON-PRODUCTION, synthetic credentials for disposable test
# containers. They are written to ./secrets with restrictive permissions and
# are never printed. Re-running overwrites them; destroy the stack and volumes
# before regenerating if InfluxDB has already initialized with the old values.
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env

mkdir -p "${SECRETS_DIR}" "${TMP_DIR}" "${LOGS_DIR}" "${ARTIFACTS_DIR}" "${REPORTS_DIR}"
chmod 700 "${SECRETS_DIR}" "${TMP_DIR}" 2>/dev/null || true

rand_hex() { openssl rand -hex "$1"; }

influxdb2_username="ttk-admin"
influxdb2_password="TTK-Test-$(rand_hex 16)"
influxdb2_token="ttk2-$(rand_hex 32)"
influxdb3_token="apiv3_$(rand_hex 32)"

# umask 077 so files are created 0600 before we tighten to 0400.
(
  umask 077
  printf '%s\n' "${influxdb2_username}" > "${SECRETS_DIR}/influxdb2-admin-username"
  printf '%s\n' "${influxdb2_password}" > "${SECRETS_DIR}/influxdb2-admin-password"
  printf '%s\n' "${influxdb2_token}"    > "${SECRETS_DIR}/influxdb2-admin-token"
  printf '{"token":"%s","name":"_admin","description":"ttk test admin token"}\n' "${influxdb3_token}" \
    > "${SECRETS_DIR}/influxdb3-admin-token.json"
)
chmod 400 "${SECRETS_DIR}"/influxdb2-admin-username \
          "${SECRETS_DIR}"/influxdb2-admin-password \
          "${SECRETS_DIR}"/influxdb2-admin-token \
          "${SECRETS_DIR}"/influxdb3-admin-token.json

echo "Wrote 4 credential files to ${SECRETS_DIR} (mode 0400). Values not displayed."
echo "InfluxDB 2 org=${INFLUXDB2_ORG} bucket=${INFLUXDB2_BUCKET}; InfluxDB 3 node=${INFLUXDB3_NODE_ID}."
