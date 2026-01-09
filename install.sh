#!/bin/bash
# ppkgmgr installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/pirakansa/ppkgmgr/main/install.sh | bash
#
# Environment variables:
#   PPKGMGR_VERSION     - Version to install (default: latest)
#   PPKGMGR_INSTALL_DIR - Installation directory (default: ~/.local/bin)

set -euo pipefail

REPO="pirakansa/ppkgmgr"
BINARY_NAME="ppkgmgr"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

# Detect OS
detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux)  echo "linux" ;;
        darwin) echo "darwin" ;;
        mingw*|msys*|cygwin*) echo "windows" ;;
        *) error "Unsupported OS: $os" ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        armv7*|armv6*) echo "arm" ;;
        *) error "Unsupported architecture: $arch" ;;
    esac
}

# Get latest version from GitHub API
get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        error "Failed to fetch latest version"
    fi
    echo "$version"
}

# Main installation logic
main() {
    local os arch version install_dir archive_name url temp_dir

    os=$(detect_os)
    arch=$(detect_arch)

    # Check for unsupported combinations
    if [ "$os" = "darwin" ] && [ "$arch" = "arm" ]; then
        error "macOS ARM (32-bit) is not supported"
    fi

    # Get version
    version="${PPKGMGR_VERSION:-latest}"
    if [ "$version" = "latest" ]; then
        info "Fetching latest version..."
        version=$(get_latest_version)
    fi
    info "Installing ppkgmgr ${version}"

    # Set install directory
    install_dir="${PPKGMGR_INSTALL_DIR:-$HOME/.local/bin}"

    # Construct download URL
    archive_name="${BINARY_NAME}_${os}-${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"

    info "Downloading from ${url}"

    # Create temp directory
    temp_dir=$(mktemp -d)
    trap 'rm -rf "$temp_dir"' EXIT

    # Download and extract
    if command -v curl &> /dev/null; then
        curl -fsSL "$url" -o "${temp_dir}/${archive_name}"
    elif command -v wget &> /dev/null; then
        wget -q "$url" -O "${temp_dir}/${archive_name}"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi

    # Extract archive
    tar -xzf "${temp_dir}/${archive_name}" -C "$temp_dir"

    # Create install directory if needed
    mkdir -p "$install_dir"

    # Install binary
    if [ "$os" = "windows" ]; then
        mv "${temp_dir}/${BINARY_NAME}.exe" "${install_dir}/"
        info "Installed ${BINARY_NAME}.exe to ${install_dir}"
    else
        mv "${temp_dir}/${BINARY_NAME}" "${install_dir}/"
        chmod +x "${install_dir}/${BINARY_NAME}"
        info "Installed ${BINARY_NAME} to ${install_dir}"
    fi

    # Check if install_dir is in PATH
    if [[ ":$PATH:" != *":${install_dir}:"* ]]; then
        warn "${install_dir} is not in your PATH"
        echo ""
        echo "Add the following line to your shell configuration file:"
        echo "  export PATH=\"\$PATH:${install_dir}\""
        echo ""
    fi

    info "Installation complete! Run 'ppkgmgr ver' to verify."
}

main "$@"
