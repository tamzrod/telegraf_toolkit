#!/usr/bin/env bash
# End-to-end live Registry check against the isolated InfluxDB test stack.
#
# Starts the Toolkit backend with a throwaway master key and authentication
# token file (created under tmp/, Git-ignored), then exercises the authorized
# Registry API:
#   1. anonymous GET /profiles           -> 401 (no anonymously reachable surface)
#   2. admin POST /profiles              -> 201 (persisted)
#   3. admin GET  /profiles              -> 200, contains the created profile
#   4. viewer POST /profiles             -> 403 (denied)
#   5. admin POST /probe (InfluxDB 2)    -> 200, live edition + capabilities
#
# Secrets live only in tmp/ (mode 0600) and are never printed. The probe policy
# is the explicit test-only loopback exception; it is never a production default.
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env

GO_BIN="${GO_BIN:-go}"
REPO_ROOT="$(cd "${ROOT_DIR}/../.." && pwd)"
API_ADDR="${TTK_API_ADDR:-127.0.0.1:18085}"
API_URL="http://${API_ADDR}"

if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
  echo "BLOCKED: Go toolchain not found (${GO_BIN})." >&2
  exit 2
fi
if ! docker_cmd ps --format '{{.Names}}' 2>/dev/null | grep -q '^ttk-influxdb2$'; then
  echo "BLOCKED: ttk-influxdb2 is not running. Start the stack with ./scripts/up.sh first." >&2
  exit 2
fi

PASS=0
FAIL=0
record() {
  local status="$1" name="$2" detail="${3:-}"
  if [ "${status}" = "PASS" ]; then PASS=$((PASS + 1)); else FAIL=$((FAIL + 1)); fi
  printf '%-4s %-52s %s\n' "${status}" "${name}" "${detail}"
}

# --- throwaway key + token files (never printed) -----------------------------
mkdir -p "${TMP_DIR}"
chmod 700 "${TMP_DIR}" 2>/dev/null || true
KEY_FILE="${TMP_DIR}/e2e-master.key"
TOKENS_FILE="${TMP_DIR}/e2e-tokens.json"
DB_FILE="${TMP_DIR}/e2e-metadata.db"
ADMIN_TOKEN="e2e-admin-token-value"
VIEWER_TOKEN="e2e-viewer-token-value"

# Start from a clean database so each run is deterministic; the key and token
# files are reused so a rerun does not regenerate secrets.
rm -f "${DB_FILE}" "${DB_FILE}-wal" "${DB_FILE}-shm"

if [ ! -f "${KEY_FILE}" ]; then
  # 32 random bytes, base64url-encoded (no padding) for the key file.
  key="$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=')"
  ( umask 077
    printf '{"active_key_id":"k1","keys":{"k1":"%s"}}\n' "${key}" > "${KEY_FILE}"
  )
  unset key
fi
if [ ! -f "${TOKENS_FILE}" ]; then
  ( umask 077
    cat > "${TOKENS_FILE}" <<JSON
{"tokens":[
  {"token":"${ADMIN_TOKEN}","actor":"e2e-admin","roles":["administrator"]},
  {"token":"${VIEWER_TOKEN}","actor":"e2e-viewer","roles":["viewer"]}
]}
JSON
  )
fi
chmod 600 "${KEY_FILE}" "${TOKENS_FILE}"

api_log="${LOGS_DIR}/e2e-registry-api.log"
api_bin="${TMP_DIR}/e2e-api"
mkdir -p "${LOGS_DIR}"
# Build the binary once so the tracked PID is the actual server (not a `go run`
# wrapper whose child would keep the port and database open after teardown).
( cd "${REPO_ROOT}/backend" && "${GO_BIN}" build -o "${api_bin}" ./cmd/api )

start_api() {
  "${api_bin}" \
    -addr "${API_ADDR}" \
    -store "${DB_FILE}" \
    -key-file "${KEY_FILE}" \
    -token-file "${TOKENS_FILE}" \
    -allow-test-probe-policy >"${api_log}" 2>&1 &
  api_pid=$!
}
start_api
trap 'kill "${api_pid}" 2>/dev/null || true; wait "${api_pid}" 2>/dev/null || true' EXIT

ready=0
for _ in $(seq 1 40); do
  if curl -sS -m 2 -o /dev/null "${API_URL}/api/v1/registry/status" 2>/dev/null; then ready=1; break; fi
  sleep 1
done
if [ "${ready}" != "1" ]; then
  echo "BLOCKED: backend did not start (see ${api_log})" >&2
  exit 2
fi
# Confirm the process we started is the one answering, so a stale listener on the
# same port cannot produce misleading results.
if ! kill -0 "${api_pid}" 2>/dev/null; then
  echo "BLOCKED: a different process is already listening on ${API_ADDR}; stop it and rerun." >&2
  exit 2
fi

echo "== Live Registry API end-to-end (${API_URL}) =="

# 1. Anonymous request is rejected.
code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' "${API_URL}/api/v1/registry/profiles" || echo 000)"
if [ "${code}" = "401" ]; then
  record PASS "anonymous GET /profiles rejected" "HTTP 401"
else
  record FAIL "anonymous GET /profiles rejected" "HTTP ${code}"
fi

# 2. Seed a TEST-only credential server-side, then create a profile that
#    references it and carries the isolated endpoint. Credential writes are not
#    exposed over HTTP, so the harness seeds one through a build-tagged helper.
SEED_REF="e2e-ref-2x"
SEED_VALUE="e2e-test-token-not-a-real-secret"
( cd "${REPO_ROOT}/backend" && \
  TOOLKIT_SEED_STORE="${DB_FILE}" \
  TOOLKIT_SEED_KEY_FILE="${KEY_FILE}" \
  TOOLKIT_SEED_REF="${SEED_REF}" \
  TOOLKIT_SEED_VALUE="${SEED_VALUE}" \
  "${GO_BIN}" test -tags integration ./internal/secretstore/ -run TestSeedCredentialForHarness -count=1 >/dev/null )

create_body="{\"connection_id\":\"e2e-influx-2x\",\"display_name\":\"E2E InfluxDB 2.x\",\"family\":\"influxdb\",\"major_version\":2,\"endpoint\":\"${INFLUXDB2_URL}\",\"scope\":{\"kind\":\"organization-bucket\",\"organization\":\"acme\",\"bucket\":\"historian\"},\"capabilities\":{\"query_dialects\":[\"flux\",\"influxql\"],\"write_protocols\":[\"line-protocol\"],\"schema_discovery\":true},\"credential_ref_id\":\"${SEED_REF}\",\"mock\":false,\"note\":\"created by e2e check\"}"
code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' -X POST \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" -H 'Content-Type: application/json' \
  -d "${create_body}" "${API_URL}/api/v1/registry/profiles" || echo 000)"
if [ "${code}" = "201" ]; then
  record PASS "admin POST /profiles persists" "HTTP 201"
else
  record FAIL "admin POST /profiles persists" "HTTP ${code}"
fi

# 3. Admin lists and sees the profile.
list_body="$(curl -sS -m 5 -H "Authorization: Bearer ${ADMIN_TOKEN}" "${API_URL}/api/v1/registry/profiles" || true)"
if printf '%s' "${list_body}" | grep -q 'e2e-influx-2x'; then
  record PASS "admin GET /profiles lists the profile" "found e2e-influx-2x"
else
  record FAIL "admin GET /profiles lists the profile" "profile missing"
fi

# 4. Viewer is denied a write.
code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' -X POST \
  -H "Authorization: Bearer ${VIEWER_TOKEN}" -H 'Content-Type: application/json' \
  -d "${create_body}" "${API_URL}/api/v1/registry/profiles" || echo 000)"
if [ "${code}" = "403" ]; then
  record PASS "viewer POST /profiles denied" "HTTP 403"
else
  record FAIL "viewer POST /profiles denied" "HTTP ${code}"
fi

# 5. Admin probes the isolated InfluxDB 2 endpoint (read-only). The endpoint and
#    credential are resolved server-side from the stored profile, so the request
#    carries only the connection profile ID.
probe_body="{\"connection_id\":\"e2e-influx-2x\",\"deadline_ms\":5000}"
probe_out="$(curl -sS -m 10 -X POST \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" -H 'Content-Type: application/json' \
  -d "${probe_body}" "${API_URL}/api/v1/registry/probe" || true)"
if printf '%s' "${probe_out}" | grep -q '"edition":"influxdb-2.x-oss"'; then
  record PASS "admin POST /probe identifies InfluxDB 2" "edition=influxdb-2.x-oss"
else
  record FAIL "admin POST /probe identifies InfluxDB 2" "$(printf '%s' "${probe_out}" | cut -c1-100)"
fi
if printf '%s' "${probe_out}" | grep -q '"mock":false'; then
  record PASS "probe result is labeled live, not mock" "mock=false"
else
  record FAIL "probe result is labeled live, not mock" "unexpected result"
fi

# 5b. A caller-supplied endpoint must be rejected: the probe resolves the
#     destination server-side from the stored profile only (F-02 regression).
smuggle_body="{\"connection_id\":\"e2e-influx-2x\",\"endpoint\":\"https://attacker.example.com\",\"deadline_ms\":5000}"
code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' -X POST \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" -H 'Content-Type: application/json' \
  -d "${smuggle_body}" "${API_URL}/api/v1/registry/probe" || echo 000)"
if [ "${code}" = "400" ]; then
  record PASS "probe rejects caller-supplied endpoint" "HTTP 400"
else
  record FAIL "probe rejects caller-supplied endpoint" "HTTP ${code}"
fi

# 6. Restart the backend against the same database and confirm the profile
#    survives, proving real persistence rather than in-memory state.
kill "${api_pid}" 2>/dev/null || true
wait "${api_pid}" 2>/dev/null || true
start_api
ready=0
for _ in $(seq 1 40); do
  if curl -sS -m 2 -o /dev/null "${API_URL}/api/v1/registry/status" 2>/dev/null; then ready=1; break; fi
  sleep 1
done
if [ "${ready}" != "1" ]; then
  record FAIL "backend restart" "did not come back up"
else
  after_body="$(curl -sS -m 5 -H "Authorization: Bearer ${ADMIN_TOKEN}" "${API_URL}/api/v1/registry/profiles" || true)"
  if printf '%s' "${after_body}" | grep -q 'e2e-influx-2x'; then
    record PASS "profile survives backend restart" "found e2e-influx-2x after restart"
  else
    record FAIL "profile survives backend restart" "profile missing after restart"
  fi
fi

echo
echo "----------------------------------------"
echo "RESULT: ${PASS} passed, ${FAIL} failed"
[ "${FAIL}" -eq 0 ]