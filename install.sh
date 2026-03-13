#!/bin/bash
set -euo pipefail

REPO="alpemreelmas/kaptan"
INSTALL_DIR="${HOME}/.kaptan-agent"
BIN_DIR="${INSTALL_DIR}/bin"
CERTS_DIR="${INSTALL_DIR}/certs"
SYSTEMD_UNIT="/etc/systemd/system/kaptan-agent.service"

# --- detect OS/arch ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Installing kaptan-agent for ${OS}/${ARCH}..."

# --- download binary ---
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"tag_name": "\(.*\)".*/\1/')

BINARY_URL="https://github.com/${REPO}/releases/download/${LATEST}/kaptan-agent-${OS}-${ARCH}"

mkdir -p "${BIN_DIR}" "${CERTS_DIR}"
curl -fsSL "${BINARY_URL}" -o "${BIN_DIR}/kaptan-agent"
chmod +x "${BIN_DIR}/kaptan-agent"

echo "Binary installed to ${BIN_DIR}/kaptan-agent"

# --- write default config ---
cat > "${INSTALL_DIR}/config.yaml" <<EOF
listen_addr: ":7000"
tls:
  cert: ${CERTS_DIR}/server.crt
  key:  ${CERTS_DIR}/server.key
  ca:   ${CERTS_DIR}/ca.crt
EOF

# --- systemd unit ---
if command -v systemctl &>/dev/null; then
  cat > /tmp/kaptan-agent.service <<EOF
[Unit]
Description=kaptan-agent gRPC deployment agent
After=network.target

[Service]
Type=simple
User=$(whoami)
ExecStart=${BIN_DIR}/kaptan-agent --config ${INSTALL_DIR}/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

  if [ -w /etc/systemd/system ]; then
    cp /tmp/kaptan-agent.service "${SYSTEMD_UNIT}"
    systemctl daemon-reload
    systemctl enable kaptan-agent
    systemctl start kaptan-agent
    echo "kaptan-agent started via systemd"
  else
    echo "No write access to /etc/systemd/system — copying unit file to ${INSTALL_DIR}/"
    cp /tmp/kaptan-agent.service "${INSTALL_DIR}/kaptan-agent.service"
    echo "Run as root to install systemd service:"
    echo "  sudo cp ${INSTALL_DIR}/kaptan-agent.service ${SYSTEMD_UNIT}"
    echo "  sudo systemctl enable --now kaptan-agent"
  fi
fi

echo ""
echo "kaptan-agent installed successfully!"
echo ""
echo "Next: paste your CA certificate into ${CERTS_DIR}/ca.crt"
echo "Or run from your dev machine: m server bootstrap <name> <ssh-user@host>"
