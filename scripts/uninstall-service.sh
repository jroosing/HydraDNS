#!/bin/bash
#
# HydraDNS Uninstall Script
#
# This script removes the HydraDNS systemd service.
# Run as root or with sudo.
#
set -euo pipefail

INSTALL_DIR="/opt/hydradns"
SERVICE_FILE="/etc/systemd/system/hydradns.service"

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

# Try to detect data directory from service file
DATA_DIR=""
if [[ -f "$SERVICE_FILE" ]]; then
    DATA_DIR=$(grep "^WorkingDirectory=" "$SERVICE_FILE" 2>/dev/null | cut -d= -f2 || true)
fi

# Stop service if running
if systemctl is-active --quiet hydradns 2>/dev/null; then
    log_info "Stopping hydradns service"
    systemctl stop hydradns
fi

# Disable service
if systemctl is-enabled --quiet hydradns 2>/dev/null; then
    log_info "Disabling hydradns service"
    systemctl disable hydradns
fi

# Remove service file
if [[ -f "$SERVICE_FILE" ]]; then
    log_info "Removing systemd service file"
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload
fi

# Remove binary
if [[ -f "$INSTALL_DIR/hydradns" ]]; then
    log_info "Removing binary"
    rm -f "$INSTALL_DIR/hydradns"
fi

# Ask about data directory
if [[ -n "$DATA_DIR" ]] && [[ -d "$DATA_DIR" ]]; then
    echo ""
    read -p "Remove data directory $DATA_DIR? This will delete the database! [y/N] " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Removing data directory"
        rm -rf "$DATA_DIR"
    else
        log_info "Keeping data directory: $DATA_DIR"
    fi
fi

echo ""
log_info "Uninstall complete!"
