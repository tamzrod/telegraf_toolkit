#!/usr/bin/env bash
# Orchestrate the full isolated integration run:
#   1. generate credentials
#   2. bring up the stack and wait for readiness
#   3. run the InfluxDB 2/3 integration checks
#   4. run the Toolkit scaffold smoke test
#   5. write sanitized evidence into ./reports
#
# usage: run-all.sh            full run, leave the stack up
#        run-all.sh --teardown tear down afterwards (keep data + secrets)
#        run-all.sh --purge    tear down and delete data + secrets
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env

mode="keep"
for arg in "$@"; do
  case "${arg}" in
    --teardown) mode="teardown" ;;
    --purge) mode="purge" ;;
    *) echo "unknown option: ${arg}" >&2; exit 2 ;;
  esac
done

mkdir -p "${REPORTS_DIR}" "${LOGS_DIR}" "${ARTIFACTS_DIR}"
report="${REPORTS_DIR}/integration-$(date -u +%Y%m%dT%H%M%SZ).txt"

{
  echo "Telegraf Toolkit — isolated InfluxDB integration run"
  echo "UTC: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "Host: $(uname -srm)"
  echo "Images:"
  echo "  influxdb:2.9.1@sha256:cdd3dbdea45c2f43513abd06ee9644b6e05d2354a2b93ca829c9f07d716eaaac"
  echo "  influxdb:3.12.1-core@sha256:1c8200f75657c7586678b52570f8c78aedfa2cd42e59c238a733423aab6bd406"
  echo
} > "${report}"

echo "### 1. init secrets"
"${SCRIPT_DIR}/init-secrets.sh" 2>&1 | sanitize_stream | tee -a "${report}"

echo "### 2. bring up stack"
"${SCRIPT_DIR}/up.sh" 2>&1 | sanitize_stream | tee -a "${report}"

echo "### 3. InfluxDB integration checks"
set +e
"${SCRIPT_DIR}/run-tests.sh" 2>&1 | sanitize_stream | tee -a "${report}"
db_rc=${PIPESTATUS[0]}
set -e

echo "### 4. Toolkit smoke"
set +e
"${SCRIPT_DIR}/toolkit-smoke.sh" 2>&1 | sanitize_stream | tee -a "${report}"
tk_rc=${PIPESTATUS[0]}
set -e

{
  echo
  echo "### Summary"
  echo "influxdb checks exit: ${db_rc}"
  echo "toolkit smoke exit:   ${tk_rc}"
} | tee -a "${report}"

case "${mode}" in
  teardown) "${SCRIPT_DIR}/teardown.sh" 2>&1 | tee -a "${report}" ;;
  purge)    "${SCRIPT_DIR}/teardown.sh" --volumes --secrets 2>&1 | tee -a "${report}" ;;
esac

echo
echo "Evidence written to ${report}"
if [ "${db_rc}" -ne 0 ] || [ "${tk_rc}" -ne 0 ]; then
  echo "One or more checks failed (db=${db_rc}, toolkit=${tk_rc})."
  exit 1
fi
echo "All checks passed."
