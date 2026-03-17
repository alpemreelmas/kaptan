# reis — Agent Reference

`reis` is the gRPC server that runs as a systemd service on each VPS. It receives commands from the `kaptan` CLI, executes deploy scripts, streams output, performs health checks, and automatically triggers rollback when a health check fails.

---

## Installation

`kaptan server bootstrap` automates the entire installation process. For manual installation, use the `install.sh` script:

```bash
curl -fsSL https://raw.githubusercontent.com/alpemreelmas/kaptan/main/install.sh | bash
```

The script:
1. Downloads the latest `reis` binary from GitHub Releases
2. Installs it to `~/.reis/bin/reis`
3. Writes a default config to `~/.reis/config.yaml`
4. Creates the `~/.reis/certs/` directory
5. Writes and starts a systemd service (if `systemctl` is available)

---

## Configuration

### Config File — `~/.reis/config.yaml`

```yaml
listen_addr: ":7000"
tls:
  cert: ~/.reis/certs/server.crt   # server certificate
  key:  ~/.reis/certs/server.key   # server private key
  ca:   ~/.reis/certs/ca.crt       # CA cert to verify clients
```

| Field | Default | Description |
|-------|---------|-------------|
| `listen_addr` | `:7000` | TCP address to bind |
| `tls.cert` | — | Server TLS certificate path |
| `tls.key` | — | Server TLS private key path |
| `tls.ca` | — | CA certificate for verifying client certs |

> If the `tls` section is omitted or any path is empty, `reis` starts **without TLS** (development only — a warning is logged).

---

### Startup and Flags

```
reis [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.reis/config.yaml` | Path to config file |

If the config file is not found, `reis` starts with defaults (no TLS, `:7000`).

Structured logging via `log/slog` produces JSON-compatible output:

```
INFO  reis starting addr=:7000
INFO  mTLS enabled
INFO  listening addr=:7000
```

---

### systemd Service

Unit file written by `install.sh`:

```ini
[Unit]
Description=reis gRPC deployment agent
After=network.target

[Service]
Type=simple
User=deploy
ExecStart=/home/deploy/.reis/bin/reis --config /home/deploy/.reis/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# Service management
systemctl status reis
systemctl restart reis
journalctl -u reis -f
```

---

## Certificate Setup (Server Side)

`kaptan server bootstrap` handles this automatically. For manual setup:

```bash
# 1. Copy CA cert from developer machine
scp ~/.kaptan/certs/ca.crt deploy@server:~/.reis/certs/ca.crt

# 2. Place server.crt and server.key (signed by the same CA) in ~/.reis/certs/

# 3. Restart reis
systemctl restart reis
```

Expected file structure:
```
~/.reis/
  config.yaml
  certs/
    ca.crt       # CA certificate copied from kaptan
    server.crt   # this server's certificate
    server.key   # this server's private key
```

---

## gRPC API

Service: `agent.v1.AgentService`
Default port: `:7000`

---

### `Deploy` (server-streaming)

Runs the specified script inside `project_path` and streams output line by line.

**Request:**

| Field | Type | Description |
|-------|------|-------------|
| `project_path` | string | Absolute path to the project on the server |
| `script` | string | Script name without extension (default: `"deploy"`) |
| `dry_run` | bool | If true, prints what would run without executing |

**Stream events (`ExecEvent`):**

| Field | Type | Description |
|-------|------|-------------|
| `line` | string | One line of stdout/stderr |
| `is_stderr` | bool | True if the line came from stderr |
| `done` | bool | True on the final event |
| `exit_code` | int32 | Script exit code (only valid when `done=true`) |

**Post-deploy flow:**

After a successful deploy (`exit_code=0`), `reis` automatically runs a health check:

```
deploy.sh exits 0
    └─► GET health_url (30s timeout)
            ├─ 2xx  →  "[health] → 200 OK" streamed, deploy succeeds
            └─ other →  rollback.sh triggered automatically
                         "[rollback] ..." lines streamed
                         gRPC status: codes.Internal returned
```

**Dry-run behaviour:** `reis` sends two synthetic events and exits without touching the filesystem:
```
[dry-run] would execute: /path/.kaptan/deploy.sh
[dry-run] done
```

---

### `Rollback` (server-streaming)

Runs `.kaptan/rollback.sh` inside `project_path`. Stream events are identical to `Deploy`.

---

### `GetStatus` (unary)

Returns the health status of all services `reis` is aware of.

**`ServiceStatus` fields:**

| Field | Type | Description |
|-------|------|-------------|
| `service_name` | string | Name of the service |
| `healthy` | bool | True if the health URL returned 2xx |
| `status_code` | int32 | Last HTTP status code from the health check |

---

### `StreamLogs` (server-streaming)

Tails a remote log file and streams lines.

**Request:**

| Field | Type | Description |
|-------|------|-------------|
| `project_path` | string | Used to locate the default log file |
| `log_file` | string | Explicit log file path (overrides auto-detection) |
| `tail` | int32 | Number of lines from the end to start with (default: 50) |

---

### `GetDependencyGraph` (unary)

Parses nginx access logs and returns a service dependency graph.

**Request:**

| Field | Type | Description |
|-------|------|-------------|
| `log_file` | string | Nginx log path (default: `/var/log/nginx/access.log`) |
| `internal_domains` | []string | Glob patterns for internal edge classification (e.g. `*.internal`) |

**Response — list of `GraphEdge`:**

| Field | Type | Description |
|-------|------|-------------|
| `from` | string | Source service (derived from log filename) |
| `to` | string | Destination host |
| `status_code` | int32 | HTTP status observed |
| `error_count` | int32 | Number of 4xx/5xx responses |
| `kind` | enum | `INTERNAL` or `EXTERNAL` |

---

## Dependency Graph Parser

`reis` matches nginx access log lines with the following pattern:

```
"GET http://service-name:3000/path HTTP/1.1" 200 1234
```

- **Source** is derived from the log filename: `/var/log/nginx/my-api.access.log` → `my-api`
- **Destination** is the upstream host extracted from the request URL
- Edges are deduplicated and grouped by `(from, to, status_code)`
- An edge is classified as `INTERNAL` if the destination host matches any pattern in `internal_domains`, or if it contains no dots (e.g. `postgres`)
- `error_count` is the number of responses with status ≥ 400

---

## Health Check and Auto-Rollback

After every successful deploy, `reis` reads `.kaptan/config.yaml` inside `project_path`:

```yaml
# .kaptan/config.yaml (inside the project, on the server)
service:    my-api
health_url: http://localhost:8080/healthz
```

If `health_url` is set:
- HTTP GET is sent with a 30-second timeout
- 2xx → `[health] → 200 OK` is streamed, deploy is considered successful
- Anything else → `.kaptan/rollback.sh` is run automatically, `[rollback] ...` lines are streamed, `codes.Internal` is returned

If `health_url` is not set, the health check is skipped and the deploy is considered successful.
