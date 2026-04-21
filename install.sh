#!/bin/sh
set -eu

REPO="${REPO:-ugnoeyh/iwinv-cli}"
BINARY="${BINARY:-iwinvctl}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

main() {
    os="$(detect_os)"
    arch="$(detect_arch)"

    if [ "$os" = "windows" ]; then
        echo "Error: this installer does not support Windows."
        echo "Download a Windows release asset (*.zip) from:"
        echo "  https://github.com/${REPO}/releases/latest"
        exit 1
    fi

    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT INT TERM

    selected_asset=""
    selected_url=""
    for asset in $(asset_candidates "$os" "$arch"); do
        url="https://github.com/${REPO}/releases/latest/download/${asset}"
        if download_file "$url" "${tmpdir}/${asset}"; then
            selected_asset="$asset"
            selected_url="$url"
            break
        fi
    done

    if [ -z "$selected_asset" ]; then
        echo "Error: no matching release asset found for ${os}/${arch}" >&2
        echo "Checked release: https://github.com/${REPO}/releases/latest" >&2
        exit 1
    fi

    echo "Detected target: ${os}/${arch}"
    echo "Downloaded: ${selected_asset}"

    if download_file "${selected_url}.sha256" "${tmpdir}/${selected_asset}.sha256"; then
        echo "Verifying checksum..."
        verify_checksum "${tmpdir}/${selected_asset}" "${tmpdir}/${selected_asset}.sha256"
    else
        echo "Warning: checksum file not found. Skipping checksum verification."
    fi

    binary_path=""
    case "$selected_asset" in
        *.tar.gz|*.tgz)
            echo "Extracting package..."
            tar -xzf "${tmpdir}/${selected_asset}" -C "$tmpdir"
            binary_path="${tmpdir}/${BINARY}"
            if [ ! -f "$binary_path" ]; then
                found_path="$(find "$tmpdir" -type f -name "$BINARY" | head -n 1 || true)"
                if [ -n "$found_path" ]; then
                    binary_path="$found_path"
                fi
            fi
            ;;
        *)
            binary_path="${tmpdir}/${selected_asset}"
            ;;
    esac

    if [ ! -f "$binary_path" ]; then
        echo "Error: ${BINARY} not found in extracted archive." >&2
        exit 1
    fi

    install_binary "$binary_path" "${INSTALL_DIR}/${BINARY}"

    echo ""
    echo "Installed: ${INSTALL_DIR}/${BINARY}"
    echo "Run:"
    echo "  ${BINARY} --login"
}

detect_os() {
    case "$(uname -s)" in
        Darwin*) echo "macos" ;;
        Linux*) echo "linux" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *)
            echo "Error: unsupported OS: $(uname -s)" >&2
            exit 1
            ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)
            echo "Error: unsupported architecture: $(uname -m)" >&2
            exit 1
            ;;
    esac
}

asset_candidates() {
    os="$1"
    arch="$2"

    case "$os" in
        linux)
            echo "${BINARY}-linux-${arch}"
            echo "${BINARY}-linux-${arch}.tar.gz"
            ;;
        macos)
            echo "${BINARY}-macos-universal2"
            echo "${BINARY}-macos-${arch}"
            echo "${BINARY}-darwin-${arch}"
            echo "${BINARY}-macos-universal2.tar.gz"
            echo "${BINARY}-macos-${arch}.tar.gz"
            echo "${BINARY}-darwin-${arch}.tar.gz"
            ;;
        *)
            echo "${BINARY}-${os}-${arch}"
            echo "${BINARY}-${os}-${arch}.tar.gz"
            ;;
    esac
}

download_file() {
    url="$1"
    out="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$out" "$url" 2>/dev/null
        return $?
    fi

    if command -v wget >/dev/null 2>&1; then
        wget -q -O "$out" "$url" 2>/dev/null
        return $?
    fi

    echo "Error: curl or wget is required." >&2
    exit 1
}

verify_checksum() {
    file="$1"
    checksum_file="$2"
    expected="$(awk '{print $1}' "$checksum_file" | head -n 1)"

    if [ -z "$expected" ]; then
        echo "Error: checksum file is empty: ${checksum_file}" >&2
        exit 1
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "$file" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        actual="$(shasum -a 256 "$file" | awk '{print $1}')"
    elif command -v openssl >/dev/null 2>&1; then
        actual="$(openssl dgst -sha256 "$file" | awk '{print $NF}')"
    else
        echo "Warning: no sha256 tool found. Skipping checksum verification."
        return 0
    fi

    if [ "$expected" != "$actual" ]; then
        echo "Error: checksum mismatch." >&2
        echo "  expected: $expected" >&2
        echo "  actual:   $actual" >&2
        exit 1
    fi
}

install_binary() {
    src="$1"
    dst="$2"

    if [ -d "$INSTALL_DIR" ]; then
        writable_check="$INSTALL_DIR"
    else
        writable_check="$(dirname "$INSTALL_DIR")"
    fi

    if [ -w "$writable_check" ]; then
        mkdir -p "$INSTALL_DIR"
        cp "$src" "$dst"
        chmod +x "$dst"
        return 0
    fi

    if ! command -v sudo >/dev/null 2>&1; then
        echo "Error: write access denied to ${INSTALL_DIR} and sudo is not available." >&2
        exit 1
    fi

    sudo mkdir -p "$INSTALL_DIR"
    sudo cp "$src" "$dst"
    sudo chmod +x "$dst"
}

main "$@"
