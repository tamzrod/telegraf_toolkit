#!/usr/bin/env bash
# Run the isolated InfluxDB 2/3 integration checks and print a PASS/FAIL table.
#
# Each check prints an explicit PASS or FAIL line with a short detail. Nothing
# is written to any historian outside this Compose project, and no secret value
# is ever printed (tokens are read from files via curl -H @file or from a file
# descriptor; see lib.sh).
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env
require_docker

PASS=0
FAIL=0
record() { # record <status> <name> <detail>
  local status="$1" name="$2" detail="${3:-}"
  if [ "${status}" = "PASS" ]; then PASS=$((PASS + 1)); else FAIL=$((FAIL + 1)); fi
  printf '%-4s %-52s %s\n' "${status}" "${name}" "${detail}"
}

# --- prepare auth header files (never in argv) -------------------------------
H2="${TMP_DIR}/hdr-influxdb2"
H3="${TMP_DIR}/hdr-influxdb3"
write_auth_header "${SECRETS_DIR}/influxdb2-admin-token" "${H2}" "Token"
# InfluxDB 3 stores the token inside a JSON file; extract just the token value
# into the header file so the JSON wrapper is never sent as the credential.
write_bearer_header_from_json "${SECRETS_DIR}/influxdb3-admin-token.json" "${H3}"

echo "== InfluxDB 2 OSS (${INFLUXDB2_URL}) =="

# 1. Liveness endpoint is unauthenticated and reports the pinned version.
h2_body="$(curl -sS -m 10 "${INFLUXDB2_URL}/health" || true)"
if printf '%s' "${h2_body}" | grep -q '"status":"pass"' && printf '%s' "${h2_body}" | grep -q 'v2.9.1'; then
  record PASS "influxdb2 /health (unauthenticated, v2.9.1)" "status=pass"
else
  record FAIL "influxdb2 /health (unauthenticated, v2.9.1)" "${h2_body:0:120}"
fi

# 2. Authenticated API access with the provisioned operator token.
code="$(curl -sS -m 10 -o /dev/null -w '%{http_code}' -H @"${H2}" "${INFLUXDB2_URL}/api/v2/orgs?org=${INFLUXDB2_ORG}")"
if [ "${code}" = "200" ]; then
  record PASS "influxdb2 authenticated /api/v2/orgs" "HTTP 200"
else
  record FAIL "influxdb2 authenticated /api/v2/orgs" "HTTP ${code}"
fi

# 3. Authorization negative: a bogus token is rejected.
code="$(curl -sS -m 10 -o /dev/null -w '%{http_code}' -H "Authorization: Token invalid-token" "${INFLUXDB2_URL}/api/v2/orgs")"
if [ "${code}" = "401" ]; then
  record PASS "influxdb2 rejects invalid token" "HTTP 401"
else
  record FAIL "influxdb2 rejects invalid token" "HTTP ${code}"
fi

# 4. Seed the synthetic dataset.
code="$(curl -sS -m 20 -o /dev/null -w '%{http_code}' -X POST -H @"${H2}" -H 'Content-Type: text/plain; charset=utf-8' --data-binary @"${SEED_DIR}/dataset.lp" \
  "${INFLUXDB2_URL}/api/v2/write?org=${INFLUXDB2_ORG}&bucket=${INFLUXDB2_BUCKET}&precision=ns")"
if [ "${code}" = "204" ]; then
  record PASS "influxdb2 write line protocol" "HTTP 204"
else
  record FAIL "influxdb2 write line protocol" "HTTP ${code}"
fi

# 5. Flux query returns the expected total point count for the field. Flux
# count() is per series, so group() first to collapse to a single total.
flux_query() {
  curl -sS -m 20 -X POST -H @"${H2}" -H 'Content-Type: application/vnd.flux' -H 'Accept: application/csv' \
    --data-binary "$1" "${INFLUXDB2_URL}/api/v2/query?org=${INFLUXDB2_ORG}" | tr -d '\r'
}
out="$(flux_query 'from(bucket:"'"${INFLUXDB2_BUCKET}"'") |> range(start:1970-01-01T00:00:00Z) |> filter(fn:(r)=> r._measurement == "environment" and r._field == "used_percent") |> group() |> count()' || true)"
if printf '%s' "${out}" | grep -q ',5$'; then
  record PASS "influxdb2 Flux count(environment.used_percent)==5" "verified"
else
  record FAIL "influxdb2 Flux count(environment.used_percent)==5" "$(printf '%s' "${out}" | tail -1 | cut -c1-80)"
fi

# 6. InfluxQL 1.x-compatible endpoint works on InfluxDB 2 (documented support).
out="$(curl -sS -m 20 -G -H @"${H2}" --data-urlencode "db=${INFLUXDB2_BUCKET}" --data-urlencode 'q=SHOW MEASUREMENTS' "${INFLUXDB2_URL}/query" || true)"
if printf '%s' "${out}" | grep -q '"environment"' && printf '%s' "${out}" | grep -q '"sensor"'; then
  record PASS "influxdb2 InfluxQL /query SHOW MEASUREMENTS" "environment+sensor"
else
  record FAIL "influxdb2 InfluxQL /query SHOW MEASUREMENTS" "$(printf '%s' "${out}" | cut -c1-80)"
fi

# 7. Restart persistence: data survives a container restart.
docker_cmd restart ttk-influxdb2 >/dev/null
wait_healthy ttk-influxdb2 24
out="$(flux_query 'from(bucket:"'"${INFLUXDB2_BUCKET}"'") |> range(start:1970-01-01T00:00:00Z) |> filter(fn:(r)=> r._measurement == "environment" and r._field == "used_percent") |> group() |> count()' || true)"
if printf '%s' "${out}" | grep -q ',5$'; then
  record PASS "influxdb2 restart persistence" "5 points after restart"
else
  record FAIL "influxdb2 restart persistence" "$(printf '%s' "${out}" | tail -1 | cut -c1-80)"
fi

echo
echo "== InfluxDB 3 Core (${INFLUXDB3_URL}) =="

# 8. The API enforces auth before any request: no token -> 401.
code="$(curl -sS -m 10 -o /dev/null -w '%{http_code}' "${INFLUXDB3_URL}/health")"
if [ "${code}" = "401" ]; then
  record PASS "influxdb3 /health requires auth" "HTTP 401 without token"
else
  record FAIL "influxdb3 /health requires auth" "HTTP ${code}"
fi

# 9. Authenticated liveness with the preconfigured admin token.
body="$(curl -sS -m 10 -H @"${H3}" "${INFLUXDB3_URL}/health" || true)"
if [ "${body}" = "OK" ]; then
  record PASS "influxdb3 authenticated /health" "OK"
else
  record FAIL "influxdb3 authenticated /health" "$(printf '%s' "${body}" | cut -c1-80)"
fi

# 10. Authorization negative.
code="$(curl -sS -m 10 -o /dev/null -w '%{http_code}' -H 'Authorization: Bearer invalid-token' "${INFLUXDB3_URL}/health")"
if [ "${code}" = "401" ]; then
  record PASS "influxdb3 rejects invalid token" "HTTP 401"
else
  record FAIL "influxdb3 rejects invalid token" "HTTP ${code}"
fi

# 11. Create the test database over the documented HTTP configuration API
# (avoids putting the token in a CLI argv). Idempotent: "already exists" is OK.
create_out="$(curl -sS -m 20 -X POST -H @"${H3}" -H 'Content-Type: application/json' \
  -d '{"db":"ttk_test_db"}' "${INFLUXDB3_URL}/api/v3/configure/database" -w '|%{http_code}' || true)"
create_code="${create_out##*|}"
if [ "${create_code}" = "200" ] || printf '%s' "${create_out}" | grep -qi 'already exists'; then
  record PASS "influxdb3 create database ttk_test_db" "HTTP ${create_code}"
else
  record FAIL "influxdb3 create database ttk_test_db" "$(printf '%s' "${create_out}" | cut -c1-80)"
fi

# 12. Seed the synthetic dataset over the v3 write_lp API.
code="$(curl -sS -m 20 -o /dev/null -w '%{http_code}' -X POST -H @"${H3}" -H 'Content-Type: text/plain; charset=utf-8' --data-binary @"${SEED_DIR}/dataset.lp" \
  "${INFLUXDB3_URL}/api/v3/write_lp?db=ttk_test_db&precision=nanosecond")"
if [ "${code}" = "204" ]; then
  record PASS "influxdb3 write_lp line protocol" "HTTP 204"
else
  record FAIL "influxdb3 write_lp line protocol" "HTTP ${code}"
fi

# 13. SQL query returns the expected row count.
out="$(curl -sS -m 20 -H @"${H3}" -H 'Content-Type: application/json' \
  "${INFLUXDB3_URL}/api/v3/query_sql" \
  -d '{"db":"ttk_test_db","q":"select count(*) as c from environment","format":"json"}' || true)"
if printf '%s' "${out}" | grep -q '"c":5'; then
  record PASS "influxdb3 SQL count(environment)==5" "verified"
else
  record FAIL "influxdb3 SQL count(environment)==5" "$(printf '%s' "${out}" | cut -c1-80)"
fi

# 14. InfluxQL-compatible endpoint on InfluxDB 3 Core.
out="$(curl -sS -m 20 -H @"${H3}" -H 'Content-Type: application/json' \
  "${INFLUXDB3_URL}/api/v3/query_influxql" \
  -d '{"db":"ttk_test_db","q":"SHOW MEASUREMENTS","format":"json"}' || true)"
if printf '%s' "${out}" | grep -q 'environment'; then
  record PASS "influxdb3 InfluxQL query_influxql SHOW MEASUREMENTS" "environment present"
else
  record FAIL "influxdb3 InfluxQL query_influxql SHOW MEASUREMENTS" "$(printf '%s' "${out}" | cut -c1-80)"
fi

# 15. Restart persistence for InfluxDB 3.
docker_cmd restart ttk-influxdb3 >/dev/null
wait_healthy ttk-influxdb3 24
out="$(curl -sS -m 20 -H @"${H3}" -H 'Content-Type: application/json' \
  "${INFLUXDB3_URL}/api/v3/query_sql" \
  -d '{"db":"ttk_test_db","q":"select count(*) as c from environment","format":"json"}' || true)"
if printf '%s' "${out}" | grep -q '"c":5'; then
  record PASS "influxdb3 restart persistence" "5 rows after restart"
else
  record FAIL "influxdb3 restart persistence" "$(printf '%s' "${out}" | cut -c1-80)"
fi

echo
echo "== InfluxDB 3 field-type fidelity =="
# 16. Integer field round-trips as an integer (preserved field type).
out="$(curl -sS -m 20 -H @"${H3}" -H 'Content-Type: application/json' \
  "${INFLUXDB3_URL}/api/v3/query_sql" \
  -d '{"db":"ttk_test_db","q":"select \"up\" from environment order by time limit 1","format":"json"}' || true)"
if printf '%s' "${out}" | grep -q '"up":1'; then
  record PASS "influxdb3 integer field type preserved" "up=1"
else
  record FAIL "influxdb3 integer field type preserved" "$(printf '%s' "${out}" | cut -c1-80)"
fi

echo
echo "----------------------------------------"
echo "RESULT: ${PASS} passed, ${FAIL} failed"
[ "${FAIL}" -eq 0 ]
