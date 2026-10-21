#!/usr/bin/env bash
# Bring up the isolated InfluxDB 2/3 test stack and wait for readiness.
set -eu

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
load_env
require_docker

ensure_test_credentials

# The containers run as HOST_UID; make the bind-mounted data dirs writable by it.
mkdir -p "${ROOT_DIR}/data/influxdb2" "${ROOT_DIR}/data/influxdb2-config" \
         "${ROOT_DIR}/data/influxdb3" "${ROOT_DIR}/data/influxdb3-plugins"

echo "Starting isolated test stack (project telegraf-toolkit-influxdb-test)..."
compose up -d

echo "Waiting for readiness..."
wait_healthy ttk-influxdb2 48 || { compose logs --no-log-prefix influxdb2 | sanitize_stream | tail -40; exit 1; }
wait_healthy ttk-influxdb3 48 || { compose logs --no-log-prefix influxdb3 | sanitize_stream | tail -40; exit 1; }

echo "Both containers are healthy:"
docker_cmd ps --filter "name=ttk-influxdb" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
echo "Endpoints: InfluxDB2 ${INFLUXDB2_URL}  InfluxDB3 ${INFLUXDB3_URL}"
