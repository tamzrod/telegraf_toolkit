#!/usr/bin/env bash
# Authenticated real-backend smoke for the P3-003 schema client.
#
# Starts the Toolkit backend with a throwaway master key and admin token,
# seeds a read credential server-side, creates a profile bound to the isolated
# InfluxDB 2 bucket, verifies the fixed GET schema routes over HTTP, then runs
# the typed browser client against the same API (no browser driver is
# installed). Tokens live only in tmp/ or the child environment and are never
# printed or placed in argv.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env

GO_BIN="${GO_BIN:-go}"
NPM_BIN="${NPM_BIN:-npx}"
REPO_ROOT="$(cd "${ROOT_DIR}/../.." && pwd)"
API_ADDR="${TTK_API_ADDR:-127.0.0.1:18087}"
API_URL="http://${API_ADDR}"
PROFILE_ID="ui-smoke-influx-2x"
REF_ID="ui-smoke-ref-2x"

if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
  echo "BLOCKED: Go toolchain not found (${GO_BIN})." >&2
  exit 2
fi
if ! command -v "${NPM_BIN}" >/dev/null 2>&1; then
  echo "BLOCKED: npx is not available; cannot run the typed client smoke." >&2
  exit 2
fi
if ! docker_cmd ps --format '{{.Names}}' 2>/dev/null | grep -q '^ttk-influxdb2$'; then
  echo "BLOCKED: ttk-influxdb2 is not running. Start the stack with ./scripts/up.sh first." >&2
  exit 2
fi
if [ ! -r "${SECRETS_DIR}/influxdb2-admin-token" ]; then
  echo "BLOCKED: the InfluxDB 2 admin token is missing or unreadable." >&2
  exit 2
fi

PASS=0
FAIL=0
record() {
  local status="$1" name="$2" detail="${3:-}"
  if [ "${status}" = "PASS" ]; then PASS=$((PASS + 1)); else FAIL=$((FAIL + 1)); fi
  printf '%-4s %-56s %s\n' "${status}" "${name}" "${detail}"
}

mkdir -p "${TMP_DIR}"
chmod 700 "${TMP_DIR}" 2>/dev/null || true
KEY_FILE="${TMP_DIR}/ui-smoke-master.key"
TOKENS_FILE="${TMP_DIR}/ui-smoke-tokens.json"
DB_FILE="${TMP_DIR}/ui-smoke-metadata.db"
TOKEN_FILE="${TMP_DIR}/ui-smoke-admin-token"
AUTH_HEADER="${TMP_DIR}/ui-smoke-auth-header"
ADMIN_TOKEN="ui-smoke-admin-token-value"

rm -f "${DB_FILE}" "${DB_FILE}-wal" "${DB_FILE}-shm" "${AUTH_HEADER}" "${TOKEN_FILE}"

if [ ! -f "${KEY_FILE}" ]; then
  key="$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')"
  ( umask 077
    printf '{"active_key_id":"k1","keys":{"k1":"%s"}}\n' "${key}" > "${KEY_FILE}"
  )
  unset key
fi

( umask 077
  cat > "${TOKENS_FILE}" <<JSON
{"tokens":[
  {"token":"${ADMIN_TOKEN}","actor":"ui-smoke-admin","roles":["administrator"]}
]}
JSON
)
( umask 077
  printf '%s\n' "${ADMIN_TOKEN}" > "${TOKEN_FILE}"
)
write_auth_header "${TOKEN_FILE}" "${AUTH_HEADER}" Bearer
chmod 600 "${KEY_FILE}" "${TOKENS_FILE}" "${TOKEN_FILE}" "${AUTH_HEADER}"

api_log="${LOGS_DIR}/ui-smoke-api.log"
api_bin="${TMP_DIR}/ui-smoke-api"
mkdir -p "${LOGS_DIR}"
( cd "${REPO_ROOT}/backend" && "${GO_BIN}" build -o "${api_bin}" ./cmd/api )

"${api_bin}" \
  -addr "${API_ADDR}" \
  -store "${DB_FILE}" \
  -key-file "${KEY_FILE}" \
  -token-file "${TOKENS_FILE}" \
  -allow-test-probe-policy >"${api_log}" 2>&1 &
api_pid=$!
cleanup() {
  kill "${api_pid}" 2>/dev/null || true
  wait "${api_pid}" 2>/dev/null || true
  rm -f "${AUTH_HEADER}" "${TOKEN_FILE}"
}
trap cleanup EXIT

ready=0
for _ in $(seq 1 40); do
  if curl -sS -m 2 -o /dev/null "${API_URL}/api/v1/registry/status" 2>/dev/null; then ready=1; break; fi
  sleep 1
done
if [ "${ready}" != "1" ]; then
  echo "BLOCKED: backend did not start (see ${api_log})" >&2
  exit 2
fi
if ! kill -0 "${api_pid}" 2>/dev/null; then
  echo "BLOCKED: another process is listening on ${API_ADDR}; stop it and rerun." >&2
  exit 2
fi

echo "== P3-003 authenticated real-backend schema smoke (${API_URL}) =="

code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' "${API_URL}/api/v1/registry/profiles" || echo 000)"
if [ "${code}" = "401" ]; then
  record PASS "anonymous GET /profiles rejected" "HTTP 401"
else
  record FAIL "anonymous GET /profiles rejected" "HTTP ${code}"
fi

INFLUX_TOKEN="$(cat "${SECRETS_DIR}/influxdb2-admin-token")"
( cd "${REPO_ROOT}/backend" && \
  TOOLKIT_SEED_STORE="${DB_FILE}" \
  TOOLKIT_SEED_KEY_FILE="${KEY_FILE}" \
  TOOLKIT_SEED_REF="${REF_ID}" \
  TOOLKIT_SEED_VALUE="${INFLUX_TOKEN}" \
  "${GO_BIN}" test -tags integration ./internal/secretstore/ -run TestSeedCredentialForHarness -count=1 >/dev/null )
unset INFLUX_TOKEN

create_body="{\"connection_id\":\"${PROFILE_ID}\",\"display_name\":\"UI smoke InfluxDB 2.x\",\"family\":\"influxdb\",\"major_version\":2,\"endpoint\":\"${INFLUXDB2_URL}\",\"scope\":{\"kind\":\"organization-bucket\",\"organization\":\"${INFLUXDB2_ORG}\",\"bucket\":\"${INFLUXDB2_BUCKET}\"},\"capabilities\":{\"query_dialects\":[\"influxql\",\"flux\"],\"write_protocols\":[\"line-protocol\"],\"schema_discovery\":true},\"credential_ref_id\":\"${REF_ID}\",\"mock\":false,\"note\":\"created by the P3-003 UI smoke\"}"
code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' -X POST \
  -H @"${AUTH_HEADER}" -H 'Content-Type: application/json' \
  -d "${create_body}" "${API_URL}/api/v1/registry/profiles" || echo 000)"
if [ "${code}" = "201" ]; then
  record PASS "admin creates the isolated InfluxDB 2 profile" "HTTP 201"
else
  record FAIL "admin creates the isolated InfluxDB 2 profile" "HTTP ${code}"
fi

measurements_file="${TMP_DIR}/ui-smoke-measurements.json"
curl -sS -m 10 -H @"${AUTH_HEADER}" \
  "${API_URL}/api/v1/registry/schema/measurements?connection_id=${PROFILE_ID}" \
  -o "${measurements_file}" || true
if grep -q '"environment"' "${measurements_file}" && \
   grep -q '"sensor"' "${measurements_file}" && \
   grep -q '"outcome":"supported"' "${measurements_file}" && \
   grep -q '"mock":false' "${measurements_file}"; then
  record PASS "fixed GET measurements route is live and supported" "environment+sensor"
else
  record FAIL "fixed GET measurements route is live and supported" "$(cut -c1-120 "${measurements_file}")"
fi

fields_file="${TMP_DIR}/ui-smoke-fields.json"
curl -sS -m 10 -H @"${AUTH_HEADER}" \
  "${API_URL}/api/v1/registry/schema/fields?connection_id=${PROFILE_ID}&measurement=sensor" \
  -o "${fields_file}" || true
if grep -q '"value"' "${fields_file}" && grep -q '"kind":"fields"' "${fields_file}"; then
  record PASS "fixed GET field-keys route returns the observed field" "value"
else
  record FAIL "fixed GET field-keys route returns the observed field" "$(cut -c1-120 "${fields_file}")"
fi

tags_file="${TMP_DIR}/ui-smoke-tags.json"
curl -sS -m 10 -H @"${AUTH_HEADER}" \
  "${API_URL}/api/v1/registry/schema/tags?connection_id=${PROFILE_ID}&measurement=sensor" \
  -o "${tags_file}" || true
if grep -q '"host"' "${tags_file}" && grep -q '"sensor_id"' "${tags_file}" && \
   grep -q '"kind":"tags"' "${tags_file}"; then
  record PASS "fixed GET tag-keys route returns observed tags" "host+sensor_id"
else
  record FAIL "fixed GET tag-keys route returns observed tags" "$(cut -c1-120 "${tags_file}")"
fi

echo "   running the typed client against the live API (token supplied via the environment)"
if (
  cd "${REPO_ROOT}/ui" && \
  TTK_LIVE_API_URL="${API_URL}" \
  TTK_LIVE_API_TOKEN="${ADMIN_TOKEN}" \
  TTK_LIVE_PROFILE_ID="${PROFILE_ID}" \
  "${NPM_BIN}" vitest run src/api/schema.live.integration.test.ts
); then
  record PASS "typed client reads the live profile and typed schema groups" "Vitest live integration"
else
  record FAIL "typed client reads the live profile and typed schema groups" "Vitest live integration"
fi

echo
echo "----------------------------------------"
echo "RESULT: ${PASS} passed, ${FAIL} failed"
[ "${FAIL}" -eq 0 ]
