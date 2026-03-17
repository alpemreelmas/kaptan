# kaptan — CLI Reference

`kaptan` is the command-line tool that runs on your developer machine. It connects to the `reis` agent on a VPS over mTLS/gRPC and sends deploy, rollback, log, and monitoring commands.

---

## Configuration

### Global Configuration — `~/.kaptan/config.yaml`

Defines all known servers and optional graph settings. Created automatically by `kaptan server add`, or edited by hand.

```yaml
servers:
  - name: web-prod-1
    host: "1.2.3.4:7000"
    tags: ["prod", "web"]
    tls:
      cert: ~/.kaptan/certs/client.crt
      key:  ~/.kaptan/certs/client.key
      ca:   ~/.kaptan/certs/ca.crt

  - name: web-staging-1
    host: "5.6.7.8:7000"
    tags: ["staging"]
    tls:
      cert: ~/.kaptan/certs/client.crt
      key:  ~/.kaptan/certs/client.key
      ca:   ~/.kaptan/certs/ca.crt

graph:
  internal_domains:
    - "*.internal"
    - "*.svc.cluster.local"
    - "localhost"
```

| Field | Description |
|-------|-------------|
| `servers[].name` | Server alias used in commands |
| `servers[].host` | Address in `host:port` format |
| `servers[].tags` | Labels for `--tag` filtering |
| `servers[].tls` | mTLS certificate paths; if empty, connects without TLS (development only) |
| `graph.internal_domains` | Hosts matching these patterns are marked as "internal" in the dependency graph |

---

### Project Configuration — `.kaptan/config.yaml`

Lives at the root of each project repo. Read by `kaptan deploy` when invoked.

```yaml
service: my-api                           # service name
server:  web-prod-1                       # server name from global config
path:    /srv/my-api                      # absolute path on the server
health_url: http://localhost:8080/healthz # checked after deploy
```

| Field | Required | Description |
|-------|----------|-------------|
| `service` | yes | Human-readable service name |
| `server` | yes | Server name from `~/.kaptan/config.yaml` |
| `path` | yes | Absolute project path on the server |
| `health_url` | no | If set, `reis` checks this URL after a successful deploy |

---

## Commands

### `kaptan deploy`

Runs `.kaptan/deploy.sh` on the configured server and streams output.

```
kaptan deploy [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--server <name>` | from `.kaptan/config.yaml` | Override the target server |
| `--dry-run` | false | Preview without executing |
| `--all` | false | Deploy to all servers matching `--tag` |
| `--tag <tag>` | — | Server filter, used together with `--all` |
| `--no-tui` | false | Plain text output instead of TUI |

**Retry logic:** Up to 3 attempts with exponential backoff on connection failures.

**Parallel deploys:** `--all --tag=prod` launches one goroutine per matching server and collects all errors.

```bash
kaptan deploy                            # deploy with TUI output
kaptan deploy --no-tui                   # plain streaming output
kaptan deploy --dry-run                  # preview only
kaptan deploy --server web-staging-1    # override server
kaptan deploy --all --tag=prod          # deploy to all prod servers in parallel
```

**TUI screen:**

The deploy TUI is active by default (disable with `--no-tui`). It parses `[N/M] description` lines from your deploy script to track phases.

```
╭─────────────────────────────────────────╮
│ kaptan deploy                           │
│                                         │
│  Service    my-api                      │
│  Server     web-prod-1 (1.2.3.4:7000)  │
│  Script     deploy                      │
│                                         │
│  [1/3] Pull latest image          ✓    │
│  [2/3] Run migrations             ●    │  ← running
│  [3/3] Restart service            ·    │  ← pending
│                                         │
│  ─── log ───                           │
│  Pulling from registry...              │
╰─────────────────────────────────────────╯
```

Phase state icons: `✓` done · `●` running · `✗` failed · `·` pending

---

### `kaptan rollback`

Runs `.kaptan/rollback.sh` on the server. Output is streamed as plain text.

```
kaptan rollback [--server <name>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--server <name>` | from `.kaptan/config.yaml` | Override the target server |

---

### `kaptan status`

Health-checks all configured services. Queries all servers in parallel and renders a TUI table.

```
kaptan status [--tag <tag>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--tag <tag>` | — | Filter servers by tag |

Output columns: `Server`, `Service`, `Healthy` (✓/✗), `HTTP Status Code`.

---

### `kaptan logs`

Streams logs from a remote service.

```
kaptan logs [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--server <name>` | from `.kaptan/config.yaml` | Target server |
| `--tail <n>` | 50 | Number of lines from the end to start streaming from |
| `--file <path>` | auto | Explicit log file path on the server |

---

### `kaptan graph`

Fetches and displays the service dependency graph derived from nginx access logs.

```
kaptan graph [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--server <name>` | from `.kaptan/config.yaml` | Target server |
| `--log-file <path>` | `/var/log/nginx/access.log` | Access log path on the server |

Displays an interactive TUI. Press `q` or `Ctrl+C` to exit.

```
╭──────────────────────────────────────────────╮
│ kaptan graph — web-prod-1                    │
│                                              │
│ my-api                                       │
│     ├─[200]──► auth-service                  │
│     ├─[200]──► postgres.internal             │
│     └─[503]──► payment-api  ← 12 err/5min   │
│                                              │
│ (q to quit)                                  │
╰──────────────────────────────────────────────╯
```

Graph legend:
- Green `[2xx]` — successful upstream call
- Red `[4xx/5xx]` — error response, with error count per 5-minute window
- Internal edges show service names; external edges show the full hostname

---

### `kaptan cert init`

Generates a self-signed CA and client certificate pair for mTLS.

```
kaptan cert init
```

Creates:
```
~/.kaptan/certs/
  ca.crt       # CA certificate (copy to server)
  ca.key       # CA private key (keep secret)
  client.crt   # client certificate
  client.key   # client private key
```

Algorithm: ECDSA P-256. After running, follow the printed next steps to bootstrap the agent.

---

### `kaptan cert rotate`

Re-generates the client certificate using the existing CA.

```
kaptan cert rotate --server <name>
```

| Flag | Required | Description |
|------|----------|-------------|
| `--server <name>` | yes | Server name (informational) |

Overwrites `~/.kaptan/certs/client.{crt,key}`. Reconnect after rotating — the old client cert will be rejected.

---

### `kaptan server add`

Registers a server in `~/.kaptan/config.yaml`.

```
kaptan server add <name> <host:port>
```

```bash
kaptan server add web-prod-1 1.2.3.4:7000
```

---

### `kaptan server bootstrap`

Installs the `reis` binary on a VPS via SSH, copies the CA certificate, and sets up the agent config.

```
kaptan server bootstrap <name> <ssh-user@host>
```

What it does:
1. SSH into the server
2. Runs the remote `install.sh` script (downloads the `reis` binary)
3. Copies `~/.kaptan/certs/ca.crt` to `~/.reis/certs/` on the server
4. Restarts `reis` as a systemd service

```bash
kaptan server bootstrap web-prod-1 deploy@1.2.3.4
```

---

## Certificate Setup Flow

```bash
# Generate certificates
kaptan cert init

# Install agent and copy ca.crt
kaptan server bootstrap web-prod-1 user@1.2.3.4

# Register server
kaptan server add web-prod-1 1.2.3.4:7000
```

**Rotation:**
```bash
kaptan cert rotate --server web-prod-1
# Old client cert is rejected — reconnect with the new one
```
