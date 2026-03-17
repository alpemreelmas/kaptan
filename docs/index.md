# Kaptan — Overview

Kaptan is a centralised microservice deployment tool for Forge-managed VPS instances. It replaces SSH-based deploys with a secure gRPC + mTLS channel. Deployment logic lives entirely in `.kaptan/deploy.sh` inside each project repo — kaptan only executes scripts and streams output back to the CLI.

---

## Components

| Component | Binary | Runs On | Role |
|-----------|--------|---------|------|
| **kaptan** | `bin/kaptan` | Developer machine | CLI — sends commands, displays output |
| **reis** | `bin/reis` | VPS server | gRPC server — executes scripts, streams logs |

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│  Developer machine                                        │
│                                                           │
│   kaptan (CLI)  ──mTLS/gRPC──►  reis (VPS)               │
│       │                             │                     │
│       │  DeployRequest              │  .kaptan/deploy.sh  │
│       │  RollbackRequest            │  .kaptan/rollback.sh│
│       │  StatusRequest              │  health checks      │
│       │  LogRequest                 │  systemd/app logs   │
│       │  GraphRequest               │  nginx log parser   │
│       │                             │                     │
│       ◄──── streaming ExecEvents ───┘                     │
└──────────────────────────────────────────────────────────┘
```

**Key design principles:**
- Deploy logic stays in your repo (`.kaptan/deploy.sh`), not in kaptan itself.
- All communication is mTLS-authenticated gRPC — no SSH needed at runtime.
- `reis` is a single stateless binary; no database, no state files.

---

## mTLS Certificate Model

Kaptan uses mutual TLS — both client (kaptan) and server (reis) authenticate with certificates signed by the same CA.

```
CA (ca.crt / ca.key)
  ├── signs → server.crt  (on the VPS, used by reis)
  └── signs → client.crt  (on your machine, used by kaptan)
```

The CA key (`ca.key`) never needs to leave your machine. The CA cert (`ca.crt`) is the only file that must be present on both sides.

---

## Quick Start (end-to-end)

```bash
# 1. Build the CLI and put it in PATH
go build -o ~/bin/kaptan ./cli

# 2. Generate mTLS certificates
kaptan cert init

# 3. Bootstrap reis on your VPS (copies ca.crt, installs binary)
kaptan server bootstrap web-prod-1 deploy@1.2.3.4

# 4. Register the server in global config
kaptan server add web-prod-1 1.2.3.4:7000

# 5. In your project repo, create .kaptan/config.yaml
cat > .kaptan/config.yaml <<EOF
service: my-api
server:  web-prod-1
path:    /srv/my-api
health_url: http://localhost:8080/healthz
EOF

# 6. Create .kaptan/deploy.sh
cat > .kaptan/deploy.sh <<'EOF'
#!/usr/bin/env bash
set -e
echo "[1/3] Pull latest image"
docker pull myrepo/my-api:latest

echo "[2/3] Run migrations"
docker run --rm myrepo/my-api:latest ./migrate

echo "[3/3] Restart service"
systemctl restart my-api
EOF
chmod +x .kaptan/deploy.sh

# 7. Deploy
kaptan deploy

# Check service health across all prod servers
kaptan status --tag=prod

# Stream logs
kaptan logs --tail=100

# View dependency graph
kaptan graph

# Roll back if needed
kaptan rollback
```

---

## Deploy Script Protocol

`reis` executes `.kaptan/deploy.sh` (and `.kaptan/rollback.sh`) inside `project_path` on the server. Scripts receive:
- Working directory set to `project_path`
- Stdout and stderr both streamed back line-by-line to the CLI

**Phase reporting** (optional but recommended):
```bash
echo "[N/M] Phase description"
```
The TUI parses this pattern and renders a progress list. Plain scripts without phase lines work fine — output is shown in the log panel.

**Exit codes:**
- `0` — success (health check runs next)
- non-zero — deploy fails; auto-rollback is NOT triggered (rollback is only triggered by a failed health check)

---

## Building from Source

Requirements: Go 1.22+, `buf` (for proto regeneration only)

```bash
git clone https://github.com/alpemreelmas/kaptan
cd kaptan

# Build everything
make build
# Output: bin/kaptan  bin/reis

# CLI only
make cli

# Agent only
make agent

# (Optional) Regenerate protobuf
cd proto && buf generate
```

The repo is a Go workspace (`go.work`) with three modules:

| Module | Path |
|--------|------|
| `github.com/alpemreelmas/kaptan/proto` | `proto/gen/` |
| `github.com/alpemreelmas/kaptan/agent` | `agent/` |
| `github.com/alpemreelmas/kaptan/cli` | `cli/` |

---

## Further Reading

- [kaptan.md](kaptan.md) — CLI commands, configuration, TUI screens
- [reis.md](reis.md) — Agent setup, configuration, gRPC API
