#!/usr/bin/env bash
# Live authorized schema-discovery integration check (P3-002).
#
# Runs the gated Go integration tests that exercise the GET-only, fixed-SHOW
# metadata read through the authorized Registry HTTP surface against the
# isolated InfluxDB 2/3 containers started by up.sh.
#
# The read path never writes: every deployment request is a GET, and the only
# writes in the harness are the synthetic seed data written by run-tests.sh
# before this check. Tokens are read from the Git-ignored secrets/ files and
# passed to the test through the environment; they are never printed and never
# placed in argv.
#
# It refuses to run unless the isolated stack is up and the synthetic data has
# been seeded, and it never contacts a host other than the loopback endpoints
# of the harness.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env

GO_BIN="${GO_BIN:-go}"
REPO_ROOT="$(cd "${ROOT_DIR}/../.." && pwd)"
# The v3 harness creates this database in run-tests.sh; it is derived from the
# v2 bucket name. Allow an explicit override for a different synthetic setup.
INFLUXDB3_DATABASE="${INFLUXDB3_DATABASE:-ttk_test_db}"

if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
  echo "BLOCKED: Go toolchain not found (${GO_BIN}). Install Go 1.27.3 per docs/DEVELOPER_SETUP.md." >&2
  exit 2
fi

for name in ttk-influxdb2 ttk-influxdb3; do
  if ! docker_cmd ps --format '{{.Names}}' 2>/dev/null | grep -q "^${name}$"; then
    echo "BLOCKED: ${name} is not running. Start the stack with ./scripts/up.sh first." >&2
    exit 2
  fi
done

for secret in \
  "${SECRETS_DIR}/influxdb2-admin-token" \
  "${SECRETS_DIR}/influxdb3-admin-token.json"; do
  if [ ! -r "${secret}" ]; then
    echo "BLOCKED: test credential ${secret} is missing or unreadable." >&2
    exit 2
  fi
done

echo "== Live authorized schema discovery (GET-only metadata reads) =="
echo "   InfluxDB 2: ${INFLUXDB2_URL} (org ${INFLUXDB2_ORG}, bucket ${INFLUXDB2_BUCKET})"
echo "   InfluxDB 3: ${INFLUXDB3_URL} (database ${INFLUXDB3_DATABASE})"

set +e
(
  cd "${REPO_ROOT}/backend" && \
  TOOLKIT_LIVE_PROBE=1 \
  TOOLKIT_INFLUXDB2_URL="${INFLUXDB2_URL}" \
  TOOLKIT_INFLUXDB2_TOKEN_FILE="${SECRETS_DIR}/influxdb2-admin-token" \
  TOOLKIT_INFLUXDB2_ORG="${INFLUXDB2_ORG}" \
  TOOLKIT_INFLUXDB2_BUCKET="${INFLUXDB2_BUCKET}" \
  TOOLKIT_INFLUXDB3_URL="${INFLUXDB3_URL}" \
  TOOLKIT_INFLUXDB3_TOKEN_FILE="${SECRETS_DIR}/influxdb3-admin-token.json" \
  TOOLKIT_INFLUXDB3_DATABASE="${INFLUXDB3_DATABASE}" \
  "${GO_BIN}" test -tags integration ./internal/registryapi/ -run IntegrationSchemaDiscovery -count=1 -v 2>&1 | sanitize_stream
)
status=$?
set -e

echo
echo "----------------------------------------"
if [ "${status}" -eq 0 ]; then
  echo "RESULT: live schema discovery PASSED"
else
  echo "RESULT: live schema discovery FAILED (exit ${status})"
fi
exit "${status}"
