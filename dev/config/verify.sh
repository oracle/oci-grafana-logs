#!/usr/bin/env bash
#
# Verifies the oci-logs-datasource plugin across multiple Grafana versions.
#
# Tests three levels:
#   1. Plugin loaded (appears in /api/plugins)
#   2. Health check passes (datasource can authenticate to OCI)
#   3. Data query returns results (dashboard panels get real data)
#
# Usage:
#   docker compose up -d
#   ./verify.sh          # plugin load + health check
#   ./verify.sh --data   # also test data queries
#
# Prerequisites for --data:
#   - SSH tunnel to OCI instance running (started via test.sh --tunnel)
#   - Provisioning directory mounted with datasource + dashboard configs

set -euo pipefail

# Source .env if it exists (for OCI_COMPARTMENT_OCID, OCI_REGION_1, etc.)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEV_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
if [ -f "$DEV_DIR/.env" ]; then
    set -a
    source "$DEV_DIR/.env"
    set +a
fi

PLUGIN_ID="oci-logs-datasource"
MAX_WAIT=90
POLL_INTERVAL=3
TEST_DATA=false

if [[ "${1:-}" == "--data" ]]; then
    TEST_DATA=true
    if [ "${OCI_COMPARTMENT_OCID:-}" = "" ] || [[ "${OCI_COMPARTMENT_OCID:-}" == *REPLACE* ]]; then
        echo "ERROR: OCI_COMPARTMENT_OCID not set. Edit dev/.env first."
        exit 1
    fi
fi

VERSIONS=("v7.5" "v9" "v10" "v11" "v12")
PORTS=(3075 3090 3100 3110 3120)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
NC='\033[0m'

pass_count=0
fail_count=0
supported_fail_count=0
v75_failed=false
data_pass=0
data_fail=0

# v7.5 is below the declared grafanaDependency floor (>=9.0.0) and is kept
# in the matrix only for parity with oci-grafana-metrics. A v7.5 failure is
# benign; any failure on a supported version (v9-v12) is not.
record_failure() {
    fail_count=$((fail_count + 1))
    if [ "$1" = "v7.5" ]; then
        v75_failed=true
    else
        supported_fail_count=$((supported_fail_count + 1))
    fi
}

echo ""
echo -e "${BOLD}OCI Logs Plugin Verification${NC}"
echo "================================"
if [ "$TEST_DATA" = true ]; then
    echo "Mode: plugin load + health check + data query"
else
    echo "Mode: plugin load + health check (use --data for query test)"
fi
echo ""

for i in "${!VERSIONS[@]}"; do
    version="${VERSIONS[$i]}"
    port="${PORTS[$i]}"
    url="http://localhost:${port}"

    echo -e "${BOLD}Grafana ${version}${NC} (port ${port})"

    # --- Step 1: Wait for Grafana to be ready (with progress dots) ---
    printf "  %-20s " "Grafana ready:"
    elapsed=0
    ready=false
    while [ "$elapsed" -lt "$MAX_WAIT" ]; do
        code=$(curl -s -o /dev/null -w "%{http_code}" "${url}/api/health" 2>/dev/null || echo "000")
        if [ "$code" = "200" ]; then
            ready=true
            break
        fi
        printf "."
        sleep "$POLL_INTERVAL"
        elapsed=$((elapsed + POLL_INTERVAL))
    done

    if [ "$ready" = false ]; then
        printf " %b\n" "${RED}FAIL${NC} (not ready after ${MAX_WAIT}s)"
        record_failure "$version"
        echo "    last container state: $(docker compose ps --format '{{.Name}}: {{.Status}}' 2>/dev/null | grep "oci-logs-grafana-${version#v}" || echo 'not found')"
        echo "    last 5 lines of grafana logs:"
        docker compose logs --tail 5 "grafana-${version#v}" 2>&1 | sed 's/^/      /' || echo "      (no logs available)"
        echo ""
        continue
    fi
    printf " %b\n" "${GREEN}OK${NC} (${elapsed}s)"

    # --- Step 2: Check plugin loaded ---
    plugin=""
    if command -v jq &>/dev/null; then
        plugin=$(curl -s "${url}/api/plugins" 2>/dev/null | jq -r ".[] | select(.id == \"${PLUGIN_ID}\") | .id" 2>/dev/null)
    else
        plugin=$(curl -s "${url}/api/plugins" 2>/dev/null | grep -o "\"id\":\"${PLUGIN_ID}\"" 2>/dev/null || true)
    fi

    if [ -n "$plugin" ]; then
        printf "  %-20s %b\n" "Plugin loaded:" "${GREEN}PASS${NC}"
        pass_count=$((pass_count + 1))
    else
        printf "  %-20s %b\n" "Plugin loaded:" "${RED}FAIL${NC}"
        record_failure "$version"
        echo ""
        continue
    fi

    # --- Step 3: Check datasource health ---
    # Find the provisioned datasource ID
    ds_id=""
    if command -v jq &>/dev/null; then
        ds_id=$(curl -s "${url}/api/datasources" 2>/dev/null | jq -r ".[] | select(.type == \"${PLUGIN_ID}\") | .id" 2>/dev/null | head -1)
    fi

    if [ -n "$ds_id" ] && [ "$ds_id" != "null" ]; then
        health_response=$(curl -s "${url}/api/datasources/${ds_id}/health" 2>/dev/null)
        health_status=""
        health_message=""
        if command -v jq &>/dev/null; then
            health_status=$(echo "$health_response" | jq -r '.status // "unknown"' 2>/dev/null)
            health_message=$(echo "$health_response" | jq -r '.message // ""' 2>/dev/null)
        fi

        if echo "$health_status" | grep -qi "ok"; then
            printf "  %-20s %b\n" "Health check:" "${GREEN}PASS${NC}"
        else
            # Save full response so users can inspect; print a one-line hint.
            health_log="/tmp/test-health-${version}.log"
            echo "$health_response" > "$health_log" 2>/dev/null

            hint=""
            if echo "$health_message" | grep -q "NotAuthorizedOrNotFound\|Unauthorized\|Forbidden"; then
                hint="OCI permission denied (IAM)"
            elif echo "$health_message" | grep -qi "connection refused\|no such host\|i/o timeout"; then
                hint="OCI unreachable"
            elif echo "$health_message" | grep -qi "timeout"; then
                hint="OCI timeout"
            elif echo "$health_message" | grep -qi "credential\|auth"; then
                hint="OCI auth not configured"
            elif [ -n "$health_message" ] && [ "$health_message" != "null" ]; then
                hint=$(echo "$health_message" | head -c 50 | tr -d '\n\r' | sed 's/  */ /g')
                [ "${#health_message}" -gt 50 ] && hint="${hint}..."
            else
                hint="datasource not configured"
            fi

            printf "  %-20s %b %s\n" "Health check:" "${YELLOW}WARN${NC}" "— $hint"
        fi
    else
        printf "  %-20s %b %s\n" "Health check:" "${YELLOW}SKIP${NC}" "— no datasource provisioned"
    fi

    # --- Step 4: Test data query (only with --data) ---
    if [ "$TEST_DATA" = true ] && [ -n "$ds_id" ] && [ "$ds_id" != "null" ]; then
        _compartment="${OCI_COMPARTMENT_OCID}"
        _region="${OCI_REGION_1:-us-ashburn-1}"
        _search_query="search \\\"${_compartment}\\\" | sort by datetime desc"
        query_payload=$(cat <<QUERY_EOF
{
  "from": "now-1h",
  "to": "now",
  "queries": [{
    "refId": "A",
    "datasourceId": ${ds_id},
    "searchQuery": "${_search_query}",
    "tenancy": "DEFAULT/",
    "tenancyName": "DEFAULT/",
    "region": "${_region}",
    "intervalMs": 60000,
    "maxDataPoints": 100
  }]
}
QUERY_EOF
        )

        # Try /api/tsdb/query first (v7.5), fall back to /api/ds/query (v9+)
        query_response=$(curl -s -X POST "${url}/api/tsdb/query" \
            -H "Content-Type: application/json" \
            -d "$query_payload" 2>/dev/null)

        # If tsdb/query returned 404 or no results key, use ds/query
        if ! echo "$query_response" | grep -q '"results"' 2>/dev/null; then
            query_response=$(curl -s -X POST "${url}/api/ds/query" \
                -H "Content-Type: application/json" \
                -d "$query_payload" 2>/dev/null)
        fi

        has_data=false
        error_msg=""

        if command -v jq &>/dev/null; then
            error_msg=$(echo "$query_response" | jq -r '.results.A.error // empty' 2>/dev/null)

            if [ -z "$error_msg" ]; then
                # v7.5: "dataframes" (base64 Arrow, non-empty array = data)
                df_count=$(echo "$query_response" | jq '.results.A.dataframes // [] | length' 2>/dev/null || echo "0")
                # v9+: "frames" (JSON objects with data.values)
                frame_count=$(echo "$query_response" | jq '.results.A.frames // [] | length' 2>/dev/null || echo "0")

                if [ "$df_count" -gt 0 ] || [ "$frame_count" -gt 0 ]; then
                    has_data=true
                fi
            fi
        else
            if echo "$query_response" | grep -q '"dataframes"\|"frames"'; then
                has_data=true
            fi
        fi

        if [ -n "$error_msg" ]; then
            err_short=$(echo "$error_msg" | head -c 60 | tr -d '\n\r')
            [ "${#error_msg}" -gt 60 ] && err_short="${err_short}..."
            printf "  %-20s %b %s\n" "Data query:" "${RED}FAIL${NC}" "— $err_short"
            data_fail=$((data_fail + 1))
        elif [ "$has_data" = true ]; then
            # Save full response so users can inspect; extract useful summary for the line.
            query_log="/tmp/test-query-${version}.log"
            echo "$query_response" > "$query_log" 2>/dev/null

            row_count=0
            sample_lines=""
            if command -v jq &>/dev/null && [ "$frame_count" -gt 0 ]; then
                # v9+ JSON frames: row count = length of first column's values
                row_count=$(echo "$query_response" | jq -r '
                    (.results.A.frames[0].data.values[0] // []) | length
                ' 2>/dev/null || echo "0")
                # Grab up to 5 sample log lines for visual verification.
                # Prefer the "data" field (the full OCI log entry JSON) — that's the
                # field most recognizable to a human verifying against the OCI Console.
                # The plugin emits: logContent, data, oracle, subject, time (see
                # pkg/plugin/constants/constants.go). Their ordering in the frame's
                # schema array can vary across Grafana versions, so we look up by name.
                sample_lines=$(echo "$query_response" | jq -r '
                    .results.A.frames[0] as $f
                    | ($f.schema.fields | map(.name)) as $names
                    | ($names | (index("data") // index("logContent") // index("body") // index("Line") // 1)) as $cIdx
                    | ($names | (index("time") // index("timestamp") // 0)) as $tIdx
                    | [range(0; 5)
                        | ($f.data.values[$tIdx][.] // "?") as $t
                        | ($f.data.values[$cIdx][.] // "?" | tostring) as $c
                        | "\($t) │ \($c)"
                      ]
                    | .[]
                ' 2>/dev/null)
            elif [ "$df_count" -gt 0 ]; then
                # v7.5 dataframes are base64 Arrow — can't decode in pure bash
                row_count="?"
            fi

            if [ "$row_count" = "?" ]; then
                printf "  %-20s %b %s\n" "Data query:" "${GREEN}PASS${NC}" "— ${df_count} dataframe(s), v7.5 Arrow (sample preview not supported; full: $query_log)"
            elif [ "$row_count" -gt 0 ] 2>/dev/null && [ -n "$sample_lines" ]; then
                shown=$(echo "$sample_lines" | wc -l | tr -d ' ')
                printf "  %-20s %b %s\n" "Data query:" "${GREEN}PASS${NC}" "— ${row_count} rows (showing first ${shown}, full: $query_log)"
                # Print each sample line indented, truncated to ~120 chars
                echo "$sample_lines" | while IFS= read -r line; do
                    [ -z "$line" ] && continue
                    truncated=$(printf '%s' "$line" | head -c 120 | tr -d '\n\r' | sed 's/  */ /g')
                    [ "${#line}" -gt 120 ] && truncated="${truncated}..."
                    printf "    %s %s\n" "│" "$truncated"
                done
            else
                printf "  %-20s %b %s\n" "Data query:" "${GREEN}PASS${NC}" "— ${row_count} rows (no logContent field; full: $query_log)"
            fi
            data_pass=$((data_pass + 1))
        else
            printf "  %-20s %b %s\n" "Data query:" "${RED}FAIL${NC}" "— no data returned"
            data_fail=$((data_fail + 1))
        fi
    fi

    echo ""
done

echo "================================"
echo -e "${BOLD}Summary${NC}"
echo -e "  Plugin load:  ${GREEN}${pass_count} passed${NC}, ${RED}${fail_count} failed${NC}"
if [ "$TEST_DATA" = true ]; then
    echo -e "  Data query:   ${GREEN}${data_pass} passed${NC}, ${RED}${data_fail} failed${NC}"
fi
echo ""

if [ "$supported_fail_count" -eq 0 ] && [ "$data_fail" -eq 0 ]; then
    echo -e "${GREEN}All checks passed.${NC}"
    [ "$v75_failed" = true ] && echo "(v7.5 failure expected — grafanaDependency >= 9.0.0)"
    exit 0
else
    echo -e "${RED}Unexpected failures detected on supported Grafana versions (v9-v12).${NC}"
    echo "Check logs: docker compose logs <service-name>"
    exit 1
fi
