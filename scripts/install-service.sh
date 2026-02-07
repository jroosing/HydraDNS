#!/bin/bash
#
# HydraDNS Installation Script for Debian/Ubuntu
#
# Downloads the latest release from GitHub and installs as a systemd service.
# Run as root or with sudo.
#
set -euo pipefail

GITHUB_REPO="jroosing/hydradns"
INSTALL_DIR="/opt/hydradns"
DATA_DIR="/opt/hydradns"
SERVICE_FILE="/etc/systemd/system/hydradns.service"

# Detect the calling user (the one who ran sudo)
if [[ -n "${SUDO_USER:-}" ]]; then
    INSTALL_USER="$SUDO_USER"
    INSTALL_GROUP=$(id -gn "$SUDO_USER")
else
    INSTALL_USER=$(whoami)
    INSTALL_GROUP=$(id -gn)
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    log_error "This script must be run as root (use sudo)"
    exit 1
fi

log_info "Installing for user: $INSTALL_USER"

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        SUFFIX="linux-amd64"
        ;;
    aarch64)
        SUFFIX="linux-arm64"
        ;;
    armv7l)
        SUFFIX="linux-armv7"
        ;;
    *)
        log_error "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

log_info "Detected architecture: $ARCH ($SUFFIX)"

# Get version to install
VERSION="${1:-latest}"
if [[ "$VERSION" == "latest" ]]; then
    log_info "Fetching latest release version..."
    VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ -z "$VERSION" ]]; then
        log_error "Failed to determine latest version"
        exit 1
    fi
fi

log_info "Installing HydraDNS ${VERSION}"

# Download binary
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/hydradns-${SUFFIX}.tar.gz"
CHECKSUM_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/checksums.txt"

TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

log_info "Downloading from: $DOWNLOAD_URL"
curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/hydradns.tar.gz"

log_info "Verifying checksum..."
curl -fsSL "$CHECKSUM_URL" -o "$TMP_DIR/checksums.txt"
EXPECTED_SHA=$(grep "hydradns-${SUFFIX}.tar.gz" "$TMP_DIR/checksums.txt" | awk '{print $1}')
ACTUAL_SHA=$(sha256sum "$TMP_DIR/hydradns.tar.gz" | awk '{print $1}')

if [[ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]]; then
    log_error "Checksum verification failed!"
    log_error "Expected: $EXPECTED_SHA"
    log_error "Got:      $ACTUAL_SHA"
    exit 1
fi
log_info "Checksum verified"

# Extract binary
log_info "Extracting binary..."
tar -xzf "$TMP_DIR/hydradns.tar.gz" -C "$TMP_DIR"

# Create install directory
log_info "Creating install directory: $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"
chown "$INSTALL_USER:$INSTALL_GROUP" "$INSTALL_DIR"
chmod 750 "$INSTALL_DIR"

# Install binary
log_info "Installing binary to $INSTALL_DIR/hydradns"
install -m 755 "$TMP_DIR/hydradns-${SUFFIX}" "$INSTALL_DIR/hydradns"

# Create systemd service file
log_info "Creating systemd service"
cat > "$SERVICE_FILE" << EOF
[Unit]
Description=HydraDNS - DNS Forwarding Server
Documentation=https://github.com/jroosing/hydradns
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${INSTALL_USER}
Group=${INSTALL_GROUP}
ExecStart=${INSTALL_DIR}/hydradns --db ${DATA_DIR}/hydradns.db
WorkingDirectory=${DATA_DIR}
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
ReadWritePaths=${DATA_DIR}

# Allow binding to privileged ports (53)
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=hydradns

[Install]
WantedBy=multi-user.target
EOF
chmod 644 "$SERVICE_FILE"

# Reload systemd
log_info "Reloading systemd daemon"
systemctl daemon-reload

# Enable service
log_info "Enabling hydradns service"
systemctl enable hydradns

echo ""
log_info "Installation complete! HydraDNS ${VERSION} installed."
echo ""
echo "Next steps:"
echo "  1. Start the service:     sudo systemctl start hydradns"
echo "  2. Check status:          sudo systemctl status hydradns"
echo "  3. View logs:             sudo journalctl -u hydradns -f"
echo ""
echo "Configuration:"
echo "  - User:                   $INSTALL_USER"
echo "  - Install directory:      $INSTALL_DIR"
echo "  - Database:               $DATA_DIR/hydradns.db"
echo "  - Binary:                 $INSTALL_DIR/hydradns"
echo ""
echo "Default ports:"
echo "  - DNS:                    53 (UDP/TCP)"
echo "  - API:                    8080 (HTTP)"
echo ""
