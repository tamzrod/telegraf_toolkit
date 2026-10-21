#!/usr/bin/env bash
# Live credential-lifecycle integration check (P2C-007).
#
# Runs the gated Go integration tests that exercise the full credential
# replacement and revocation lifecycle through the authorized Registry HTTP
# surface against the isolated InfluxDB 2/3 containers started by up.sh:
#
#   - a replacement candidate that the deployment's public edition
#     identification would accept but whose authorization cannot be proven is
#     REFUSED, and the working credential stays in place;
#   - the real provisioned token is authorized through a protected, read-only
#     listing, so its replacement completes;
#   - revocation is refused while anything still references the credential,
#     and succeeds once nothing does.
#
# Every request the lifecycle makes against a deployment is a read-only GET
# through the live prober's verification; no historian is ever written. Tokens
# are read from the Git-ignored secrets/ files and passed to the test through
# the environment; they are never printed and never placed in argv.
#
# It refuses to run unless the isolated stack is up and reachable, and it
# never contacts a host other than the loopback endpoints of the harness.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env

GO_BIN="${GO_BIN:-go}"
REPO_ROOT="$(cd "${ROOT_DIR}/../.." && pwd)"

if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
  echo "BLOCKED: Go toolchain not found (${GO_BIN}). Install Go 1.27.3 per docs/DEVELOPER_SETUP.md." >&2
  exit 2
fi

# Require the isolated stack to be running.
if ! docker_cmd ps --format '{{.Names}}' 2>/dev/null | grep -q '^ttk-influxdb2$'; then
  echo "BLOCKED: ttk-influxdb2 is not running. Start the stack with ./scripts/up.sh first." >&2
  exit 2
fi
if ! docker_cmd ps --format '{{.Names}}' 2>/dev/null | grep -q '^ttk-influxdb3$'; then
  echo "BLOCKED: ttk-influxdb3 is not running. Start the stack with ./scripts/up.sh first." >&2
  exit 2
fi

# Confirm both credentials exist before pointing the test at them.
for secret in \
  "${SECRETS_DIR}/influxdb2-admin-token" \
  "${SECRETS_DIR}/influxdb3-admin-token.json"; do
  if [ ! -r "${secret}" ]; then
    echo "BLOCKED: test credential ${secret} is missing or unreadable." >&2
    exit 2
  fi
done

echo "== Live credential lifecycle (read-only authorization checks) =="
echo "   InfluxDB 2: ${INFLUXDB2_URL}"
echo "   InfluxDB 3: ${INFLUXDB3_URL}"

set +e
(
  cd "${REPO_ROOT}/backend" && \
  TOOLKIT_LIVE_PROBE=1 \
  TOOLKIT_INFLUXDB2_URL="${INFLUXDB2_URL}" \
  TOOLKIT_INFLUXDB2_TOKEN_FILE="${SECRETS_DIR}/influxdb2-admin-token" \
  TOOLKIT_INFLUXDB3_URL="${INFLUXDB3_URL}" \
  TOOLKIT_INFLUXDB3_TOKEN_FILE="${SECRETS_DIR}/influxdb3-admin-token.json" \
  "${GO_BIN}" test -tags integration ./internal/registryapi/ -run IntegrationCredentialLifecycle -count=1 -v 2>&1 | sanitize_stream
)
status=$?
set -e

echo
echo "----------------------------------------"
if [ "${status}" -eq 0 ]; then
  echo "RESULT: live credential lifecycle PASSED"
else
  echo "RESULT: live credential lifecycle FAILED (exit ${status})"
fi
exit "${status}"
