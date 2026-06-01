#!/bin/sh
# subs-check one-click install script
# Compatible with bash / sh / dash
# Usage: curl -fsSL https://raw.githubusercontent.com/beck-8/subs-check/master/install.sh | bash
#   or: wget -qO- https://raw.githubusercontent.com/beck-8/subs-check/master/install.sh | bash
# Acceleration: bash <(curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/beck-8/subs-check/master/install.sh) https://ghfast.top/

set -e

# ============ Configuration ============
REPO="beck-8/subs-check"
INSTALL_DIR="/opt/subs-check"
BINARY_NAME="subs-check"
SERVICE_NAME="subs-check"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
GITHUB_API="https://api.github.com/repos/${REPO}/releases/latest"
GITHUB_PROXY="${1:-}"

# ============ Runtime State ============
HAS_SYSTEMD=1
IS_UPGRADE=0

# ============ Color Output ============
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info()  { printf "${BLUE}[INFO]${NC} %s\n" "$1"; }
ok()    { printf "${GREEN}[OK]${NC} %s\n" "$1"; }
warn()  { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
error() { printf "${RED}[ERROR]${NC} %s\n" "$1"; exit 1; }

# ============ Preflight Checks ============
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        error "Please run this script as root or with sudo"
    fi
}

check_os() {
    if [ "$(uname -s)" != "Linux" ]; then
        error "This script only supports Linux"
    fi
}

check_systemd() {
    if ! command -v systemctl >/dev/null 2>&1; then
        HAS_SYSTEMD=0
        warn "systemd was not detected; service setup will be skipped and you must run it manually after installation"
    fi
}

check_existing() {
    if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        IS_UPGRADE=1
        info "Existing installation detected; upgrading"
    fi
}

check_download_tool() {
    if command -v curl >/dev/null 2>&1; then
        DOWNLOADER="curl"
    elif command -v wget >/dev/null 2>&1; then
        DOWNLOADER="wget"
    else
        error "curl or wget is required; please install one of them first"
    fi
}

# ============ Download Helpers ============
download() {
    url="$1"
    output="$2"
    if [ "$DOWNLOADER" = "curl" ]; then
        curl -fsSL -o "$output" "$url"
    else
        wget -qO "$output" "$url"
    fi
}

fetch_url() {
    url="$1"
    if [ "$DOWNLOADER" = "curl" ]; then
        curl -fsSL "$url"
    else
        wget -qO- "$url"
    fi
}

# ============ Architecture Detection ============
detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)
            ARCH="x86_64"
            ;;
        aarch64|arm64)
            ARCH="aarch64"
            ;;
        armv7*|armhf)
            ARCH="armv7"
            ;;
        i386|i686)
            ARCH="i386"
            ;;
        *)
            error "Unsupported architecture: $arch"
            ;;
    esac
    ok "Detected system architecture: $ARCH"
}

# ============ Fetch Latest Version ============
get_latest_version() {
    info "Fetching latest version..."
    LATEST_VERSION=$(fetch_url "$GITHUB_API" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    if [ -z "$LATEST_VERSION" ]; then
        error "Could not fetch the latest version; please check your network connection"
    fi
    ok "Latest version: $LATEST_VERSION"
}

# ============ Download And Install ============
install_binary() {
    FILE_NAME="${BINARY_NAME}_Linux_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/${FILE_NAME}"

    if [ -n "$GITHUB_PROXY" ]; then
        DOWNLOAD_URL="${GITHUB_PROXY}${DOWNLOAD_URL}"
    fi

    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    info "Downloading ${FILE_NAME}..."
    download "$DOWNLOAD_URL" "${TMP_DIR}/${FILE_NAME}"
    ok "Download completed"

    if [ "$IS_UPGRADE" -eq 1 ]; then
        info "Upgrading program in ${INSTALL_DIR}..."
    else
        info "Installing to ${INSTALL_DIR}..."
    fi

    mkdir -p "$INSTALL_DIR"
    tar -xzf "${TMP_DIR}/${FILE_NAME}" -C "$TMP_DIR"

    # Stop the running service during upgrades.
    if [ "$IS_UPGRADE" -eq 1 ] && [ "$HAS_SYSTEMD" -eq 1 ]; then
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            warn "Running service detected; stopping..."
            systemctl stop "$SERVICE_NAME"
        fi
    fi

    cp "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

    if [ "$IS_UPGRADE" -eq 1 ]; then
        ok "Upgrade completed: ${INSTALL_DIR}/${BINARY_NAME}"
    else
        ok "Installation completed: ${INSTALL_DIR}/${BINARY_NAME}"
    fi
}

# ============ Configure systemd ============
setup_systemd() {
    info "Configuring systemd service..."
    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Subs Check - Subscription checking and conversion tool
After=network-online.target
Wants=network-online.target
StartLimitBurst=5
StartLimitIntervalSec=60

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
Restart=always
RestartSec=10
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    ok "systemd service configured"
}

# ============ Interactive Choices ============
ask_enable() {
    printf "${YELLOW}Enable startup on boot? [Y/n]: ${NC}"
    read -r answer < /dev/tty
    case "$answer" in
        [nN]|[nN][oO])
            info "Skipped startup on boot"
            ;;
        *)
            systemctl enable "$SERVICE_NAME"
            ok "Startup on boot enabled"
            ;;
    esac
}

ask_start() {
    printf "${YELLOW}Start the service now? [Y/n]: ${NC}"
    read -r answer < /dev/tty
    case "$answer" in
        [nN]|[nN][oO])
            info "Skipped starting service"
            ;;
        *)
            systemctl start "$SERVICE_NAME"
            ok "Service started"
            ;;
    esac
}

# ============ Print Information ============
print_info() {
    printf "\n"
    printf "${GREEN}========================================${NC}\n"
    if [ "$IS_UPGRADE" -eq 1 ]; then
        printf "${GREEN} subs-check upgraded successfully!${NC}\n"
    else
        printf "${GREEN} subs-check installed successfully!${NC}\n"
    fi
    printf "${GREEN}========================================${NC}\n"
    printf "\n"
    printf "  Version:        %s\n" "$LATEST_VERSION"
    printf "  Install dir:    %s\n" "$INSTALL_DIR"
    printf "  Config file:    %s/config/config.yaml\n" "$INSTALL_DIR"

    if [ "$HAS_SYSTEMD" -eq 1 ]; then
        printf "  Service management:\n"
        printf "    Start:     systemctl start %s\n" "$SERVICE_NAME"
        printf "    Stop:      systemctl stop %s\n" "$SERVICE_NAME"
        printf "    Restart:   systemctl restart %s\n" "$SERVICE_NAME"
        printf "    Status:    systemctl status %s\n" "$SERVICE_NAME"
        printf "    Logs:      journalctl -u %s -f\n" "$SERVICE_NAME"
        printf "\n"
        printf "  Uninstall:\n"
        printf "    systemctl stop %s\n" "$SERVICE_NAME"
        printf "    systemctl disable %s\n" "$SERVICE_NAME"
        printf "    rm -rf %s %s\n" "$INSTALL_DIR" "$SERVICE_FILE"
        printf "    systemctl daemon-reload\n"
    else
        printf "\n"
        printf "${YELLOW}  systemd was not detected; run manually:${NC}\n"
        printf "    cd %s && ./%s\n" "$INSTALL_DIR" "$BINARY_NAME"
        printf "\n"
        printf "  Run in background:\n"
        printf "    cd %s && nohup ./%s > subs-check.log 2>&1 &\n" "$INSTALL_DIR" "$BINARY_NAME"
        printf "\n"
        printf "  View logs:\n"
        printf "    tail -f %s/subs-check.log\n" "$INSTALL_DIR"
        printf "\n"
        printf "  Uninstall:\n"
        printf "    rm -rf %s\n" "$INSTALL_DIR"
    fi
    printf "\n"
    printf "${YELLOW}  To change parameters, edit the config file:${NC}\n"
    printf "    %s/config/config.yaml\n" "$INSTALL_DIR"
    if [ "$HAS_SYSTEMD" -eq 1 ]; then
        printf "  Restart the service after changes: systemctl restart %s\n" "$SERVICE_NAME"
    else
        printf "  Rerun the program after changes for them to take effect\n"
    fi
    printf "\n"
}

# ============ Main Flow ============
main() {
    printf "\n"
    printf "${GREEN}========================================${NC}\n"
    printf "${GREEN} subs-check one-click install script${NC}\n"
    printf "${GREEN}========================================${NC}\n"
    printf "\n"

    check_root
    check_os
    check_systemd
    check_existing
    check_download_tool
    detect_arch
    get_latest_version
    install_binary

    if [ "$HAS_SYSTEMD" -eq 1 ]; then
        setup_systemd
        ask_enable
        # Upgrade mode asks whether to restart; new installs ask whether to start now.
        if [ "$IS_UPGRADE" -eq 1 ]; then
            printf "${YELLOW}Restart the service? [Y/n]: ${NC}"
            read -r answer < /dev/tty
            case "$answer" in
                [nN]|[nN][oO])
                    info "Skipped starting service"
                    ;;
                *)
                    systemctl restart "$SERVICE_NAME"
                    ok "Service restarted"
                    ;;
            esac
        else
            ask_start
        fi
    fi

    print_info
}

main
