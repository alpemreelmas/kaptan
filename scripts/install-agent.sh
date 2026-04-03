#!/bin/bash
set -euo pipefail

REPO="alpemreelmas/kaptan"
INSTALL_DIR="${HOME}/.reis"
BIN_DIR="${INSTALL_DIR}/bin"
CERTS_DIR="${INSTALL_DIR}/certs"
SYSTEMD_UNIT="/etc/systemd/system/reis.service"

# --- detect OS/arch ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Installing reis for ${OS}/${ARCH}..."

# --- download binary ---
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"tag_name": "\(.*\)".*/\1/')

BINARY_URL="https://github.com/${REPO}/releases/download/${LATEST}/reis-${OS}-${ARCH}"

mkdir -p "${BIN_DIR}" "${CERTS_DIR}"
curl -fsSL "${BINARY_URL}" -o "${BIN_DIR}/reis"
chmod +x "${BIN_DIR}/reis"

echo "Binary installed to ${BIN_DIR}/reis"

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
  cat > /tmp/reis.service <<EOF
[Unit]
Description=reis gRPC deployment agent
After=network.target

[Service]
Type=simple
User=$(whoami)
ExecStart=${BIN_DIR}/reis --config ${INSTALL_DIR}/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

  if [ -w /etc/systemd/system ]; then
    cp /tmp/reis.service "${SYSTEMD_UNIT}"
    systemctl daemon-reload
    systemctl enable reis
    systemctl start reis
    echo "reis started via systemd"
  else
    echo "No write access to /etc/systemd/system — copying unit file to ${INSTALL_DIR}/"
    cp /tmp/reis.service "${INSTALL_DIR}/reis.service"
    echo "Run as root to install systemd service:"
    echo "  sudo cp ${INSTALL_DIR}/reis.service ${SYSTEMD_UNIT}"
    echo "  sudo systemctl enable --now reis"
  fi
fi

echo ""
echo "reis installed successfully!"
echo ""
echo "Next: paste your CA certificate into ${CERTS_DIR}/ca.crt"
echo "Or run from your dev machine: kaptan server bootstrap <name> <ssh-user@host>"
