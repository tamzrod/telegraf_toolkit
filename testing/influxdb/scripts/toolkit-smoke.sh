#!/usr/bin/env bash
# Toolkit smoke test against the local scaffold API.
#
# Reports PASS / FAIL / NOT IMPLEMENTED separately for:
#   - the scaffold liveness endpoint /api/v1/health
#   - the authenticated live Registry endpoint (not implemented in this repo)
#
# This does NOT contact any database and does not stand in for the InfluxDB
# integration checks (see run-tests.sh). It only exercises the Toolkit backend
# as it exists in source.
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env

GO_BIN="${GO_BIN:-go}"
API_ADDR="${TTK_API_ADDR:-127.0.0.1:18080}"
API_URL="http://${API_ADDR}"
REPO_ROOT="$(cd "${ROOT_DIR}/../.." && pwd)"

if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
  echo "BLOCKED: Go toolchain not found (${GO_BIN}). Install Go 1.27.3 per docs/DEVELOPER_SETUP.md." >&2
  exit 2
fi

PASS=0; FAIL=0; NI=0
record() {
  local s="$1" n="$2" d="${3:-}"
  case "${s}" in
    PASS) PASS=$((PASS+1)) ;;
    FAIL) FAIL=$((FAIL+1)) ;;
    *)    NI=$((NI+1)) ;;
  esac
  printf '%-15s %-46s %s\n' "${s}" "${n}" "${d}"
}

echo "== Toolkit scaffold API (${API_URL}) =="

api_log="${LOGS_DIR}/toolkit-api.log"
mkdir -p "${LOGS_DIR}"
( cd "${REPO_ROOT}/backend" && "${GO_BIN}" run ./cmd/api -addr "${API_ADDR}" ) >"${api_log}" 2>&1 &
api_pid=$!
trap 'kill "${api_pid}" 2>/dev/null || true; wait "${api_pid}" 2>/dev/null || true' EXIT

# Wait for the API to accept connections.
ready=0
for _ in $(seq 1 30); do
  if curl -sS -m 2 -o /dev/null "${API_URL}/api/v1/health" 2>/dev/null; then ready=1; break; fi
  sleep 1
done
if [ "${ready}" != "1" ]; then
  record FAIL "backend /api/v1/health reachable" "API did not start (see ${api_log})"
  exit 1
fi

# 1. Liveness endpoint: 200 + the documented scaffold payload.
code="$(curl -sS -m 5 -o "${TMP_DIR}/health.json" -w '%{http_code}' "${API_URL}/api/v1/health")"
body="$(cat "${TMP_DIR}/health.json")"
if [ "${code}" = "200" ] && printf '%s' "${body}" | grep -q '"status":"ok"' \
   && printf '%s' "${body}" | grep -q '"scaffold":true' \
   && printf '%s' "${body}" | grep -q '"connected":false'; then
  record PASS "GET /api/v1/health (scaffold)" "200, scaffold=true, connected=false"
else
  record FAIL "GET /api/v1/health (scaffold)" "HTTP ${code}: $(printf '%s' "${body}" | cut -c1-80)"
fi

# 2. Method handling: POST must be 405 with an Allow header.
code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' -X POST "${API_URL}/api/v1/health")"
if [ "${code}" = "405" ]; then
  record PASS "POST /api/v1/health rejected" "HTTP 405"
else
  record FAIL "POST /api/v1/health rejected" "HTTP ${code}"
fi

# 3. Live Registry endpoint: the browser client calls /api/v1/registry/profiles,
# but the backend does not implement it. Report honestly, do not fake a result.
code="$(curl -sS -m 5 -o /dev/null -w '%{http_code}' "${API_URL}/api/v1/registry/profiles" || echo "000")"
if [ "${code}" = "404" ]; then
  record "NOT IMPLEMENTED" "GET /api/v1/registry/profiles" "HTTP 404 - no live Registry in source"
else
  record FAIL "GET /api/v1/registry/profiles" "HTTP ${code} (unexpected)"
fi

echo
echo "----------------------------------------"
echo "RESULT: ${PASS} passed, ${FAIL} failed, ${NI} not implemented"
[ "${FAIL}" -eq 0 ]
