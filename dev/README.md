# OCI Logs Plugin Dev Harness

This directory contains a local validation harness for the OCI Logs Grafana
datasource plugin. It builds the plugin, starts a Grafana compatibility matrix,
provisions the OCI Logs datasource, and optionally verifies real OCI Logging
queries through Instance Principal authentication.

The harness is local-only. It is not wired into CI.

## What It Tests

The harness validates three layers:

1. Grafana starts and responds on each local port.
2. Grafana loads the `oci-logs-datasource` backend plugin from `dist/`.
3. Optional: the datasource runs a real OCI Logging query and returns rows.

The matrix currently starts these Grafana containers:

| Version | Port | URL |
| --- | ---: | --- |
| v7.5 | 3075 | http://localhost:3075 |
| v9 | 3090 | http://localhost:3090 |
| v10 | 3100 | http://localhost:3100 |
| v11 | 3110 | http://localhost:3110 |
| v12 | 3120 | http://localhost:3120 |

`src/plugin.json` declares `grafanaDependency: >=9.0.0`. Grafana v7.5 is kept
only for parity with the OCI metrics plugin harness. A v7.5 plugin-load failure
is expected and is treated as benign; failures on v9-v12 are not.

## Prerequisites

Install or make available:

- Docker with Compose v2 (`docker compose`)
- Go and `mage`
- Node.js and Yarn v1
- `jq` for `--data` verification and response inspection
- SSH access to an OCI compute instance when testing Instance Principal auth
- Optional: OCI CLI for independent log comparison

If `mage` is installed under `~/go/bin`, export it before running the harness:

```bash
export PATH="$HOME/go/bin:$PATH"
```

Install frontend dependencies once from the repository root:

```bash
cd /path/to/oci-grafana-logs
yarn install --frozen-lockfile
```

Without `node_modules`, `yarn build` fails with `webpack: command not found`.

## Configuration

Create a local environment file:

```bash
cd /path/to/oci-grafana-logs/dev
cp .env.example .env
```

`dev/.env` is ignored by Git. Do not commit it.

| Variable | Required | Description |
| --- | --- | --- |
| `OCI_COMPARTMENT_OCID` | Yes for `--data` | Compartment OCID containing logs to query. |
| `OCI_REGION_1` | Yes | Region used by the `--data` query and the first dashboard panel. |
| `OCI_REGION_2` | Yes | Region used by the second dashboard panel. |
| `SSH_TUNNEL_HOST` | Yes for `--tunnel` | SSH host alias for an OCI compute instance. |

Example:

```env
OCI_COMPARTMENT_OCID=ocid1.compartment.oc1..example
OCI_REGION_1=us-ashburn-1
OCI_REGION_2=us-phoenix-1
SSH_TUNNEL_HOST=oci-grafana-compute-01001-iad-alpha
```

Keep `OCI_REGION_1` aligned with the tunnel host when testing real data. For
example, use `us-ashburn-1` with an IAD/alpha tunnel host and `us-phoenix-1`
with a PHX/beta tunnel host. A mismatch can make the test query a different
region than the Instance Principal path you intended to validate.

## Commands

Run all commands from `dev/`.

Show current tunnel and container state:

```bash
./test.sh --status
```

Build the plugin, start the Grafana matrix, and verify plugin load plus
datasource health:

```bash
./test.sh
```

Start the Instance Principal metadata tunnel, then build/start/verify:

```bash
./test.sh --tunnel
```

Run the full validation, including real OCI Logging queries:

```bash
./test.sh --tunnel --data
```

Stop containers and the harness-managed tunnel:

```bash
./test.sh --down
```

The script writes full command output to temporary files for debugging:

| File | Contents |
| --- | --- |
| `/tmp/test-yarn.log` | `yarn build` output |
| `/tmp/test-mage.log` | `mage -v` output |
| `/tmp/test-compose.log` | `docker compose up -d` output |
| `/tmp/test-health-v*.log` | Full datasource health response per Grafana version |
| `/tmp/test-query-v*.log` | Full datasource query response per Grafana version |

## Expected Results

A successful supported-version run should show:

```text
Grafana ready:       OK
Plugin loaded:       PASS
Data query:          PASS
```

`Health check: WARN - OCI permission denied (IAM)` can be acceptable when
`Data query: PASS`. The health check and data query validate different backend
paths:

- Health check calls Logging Management `ListLogGroups` at the tenancy/root
  scope with subtree enabled.
- Data query calls OCI Logging Search for `OCI_COMPARTMENT_OCID`.

If the Instance Principal has compartment-scoped log-search access but not
tenancy-wide log-group listing, health can warn while real log queries pass.
Treat `Data query: PASS` as the primary evidence that the plugin can pull logs.

If `Data query: FAIL`, inspect the corresponding `/tmp/test-query-v*.log` and
Grafana container logs.