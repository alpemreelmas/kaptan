# kaptan

A lightweight, mTLS-secured gRPC deployment agent for Forge-managed VPS instances.

Instead of SSH-based deploys, kaptan runs a persistent agent on each server that executes your project's `.kaptan/deploy.sh` script and streams output back to the CLI in real time.

---

## Architecture

```
Dev machine                          VPS
┌─────────────┐   gRPC + mTLS   ┌──────────────────┐
│  m (CLI)    │ ──────────────► │  kaptan-agent    │
│             │ ◄── stream ──── │  :7000           │
└─────────────┘                 └──────────────────┘
                                  runs .kaptan/deploy.sh
```

- **CLI** (`m`) — runs on your dev machine, reads `~/.kaptan/config.yaml` + per-project `.kaptan/config.yaml`
- **Agent** (`kaptan-agent`) — runs as a systemd service on each VPS, listens on `:7000`
- **mTLS** — mutual TLS with ECDSA P-256 certs; both sides verify each other

---

## Quick Start

### 1. Install the CLI

```bash
go install github.com/alpemreelmas/kaptan/cli@latest
# binary is installed as 'm'
```

Or build from source:

```bash
git clone https://github.com/alpemreelmas/kaptan
cd kaptan
make install   # installs 'm' to $GOPATH/bin
```

### 2. Generate mTLS certificates

```bash
m cert init
# creates ~/.kaptan/certs/{ca,client}.{crt,key}
```

### 3. Register your server

```bash
m server add web-prod-1 your-server-ip:7000
```

### 4. Bootstrap the agent on the VPS

```bash
m server bootstrap web-prod-1 forge@your-server-ip
# SSH-installs kaptan-agent, copies CA cert, starts systemd service
```

### 5. Set up a project

In your project repo, create `.kaptan/config.yaml`:

```yaml
service: my-app
server: web-prod-1
path: /home/forge/myapp.com
health_url: http://127.0.0.1:8000/health
```

And `.kaptan/deploy.sh`:

```bash
#!/bin/bash
set -euo pipefail
cd /home/forge/myapp.com
git pull origin main
composer install --no-dev
php artisan migrate --force
php artisan config:cache
sudo supervisorctl restart myapp
```

### 6. Deploy

```bash
m deploy              # plain output
m deploy --tui        # interactive TUI
m deploy --dry-run    # preview only
```

---

## CLI Reference

| Command | Description |
|---------|-------------|
| `m deploy` | Run `.kaptan/deploy.sh` on the configured server, stream output |
| `m deploy --all` | Deploy to all servers |
| `m deploy --tag=prod` | Deploy to servers tagged `prod` |
| `m rollback` | Run `.kaptan/rollback.sh` |
| `m status` | Health check all configured services |
| `m logs` | Stream logs from the server |
| `m graph` | Show service dependency graph (from nginx logs) |
| `m cert init` | Generate CA + client certificates |
| `m cert rotate` | Rotate certificates |
| `m server add <name> <addr>` | Register a server in global config |
| `m server bootstrap <name> <ssh>` | Install agent on VPS via SSH |

---

## Configuration

### Global config — `~/.kaptan/config.yaml`

```yaml
servers:
  web-prod-1:
    addr: "your-server-ip:7000"
    tags: ["prod"]
    tls:
      cert: ~/.kaptan/certs/client.crt
      key:  ~/.kaptan/certs/client.key
      ca:   ~/.kaptan/certs/ca.crt
  web-staging-1:
    addr: "staging-ip:7000"
    tags: ["staging"]
    tls:
      cert: ~/.kaptan/certs/client.crt
      key:  ~/.kaptan/certs/client.key
      ca:   ~/.kaptan/certs/ca.crt
```

### Project config — `.kaptan/config.yaml`

```yaml
service: my-app
server: web-prod-1
path: /home/forge/myapp.com
health_url: http://127.0.0.1:8000/health
```

---

## Agent Configuration — `~/.kaptan-agent/config.yaml`

```yaml
listen_addr: ":7000"
tls:
  cert: ~/.kaptan-agent/certs/server.crt
  key:  ~/.kaptan-agent/certs/server.key
  ca:   ~/.kaptan-agent/certs/ca.crt
```

---

## Building from Source

```bash
# Build both binaries
make build
# outputs: bin/kaptan-agent, bin/m

# Run tests
make test

# Regenerate proto (requires buf)
make proto
```

---

## How Deploys Work

1. `m deploy` reads `.kaptan/config.yaml` to find the server and project path
2. Connects to the agent via gRPC + mTLS
3. Agent executes `.kaptan/deploy.sh` inside the project `path`
4. Output is streamed line-by-line back to the CLI
5. After success, agent runs a health check against `health_url`
6. If health check fails, agent automatically triggers rollback

---

## License

MIT
