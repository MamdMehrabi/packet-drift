#!/usr/bin/env bash

set -Eeuo pipefail

readonly XRAY_VERSION="v26.3.27"
readonly REPOSITORY="XTLS/Xray-core"
readonly ASSETS_DIR="internal/xray/assets"

case "$(uname -s)/$(uname -m)" in
    Darwin/x86_64)
        platform="macos"
        architecture="64"
        directory="darwin-amd64"
        ;;
    Darwin/arm64)
        platform="macos"
        architecture="arm64-v8a"
        directory="darwin-arm64"
        ;;
    Linux/x86_64)
        platform="linux"
        architecture="64"
        directory="linux-amd64"
        ;;
    Linux/aarch64|Linux/arm64)
        platform="linux"
        architecture="arm64-v8a"
        directory="linux-arm64"
        ;;
    *)
        echo "Unsupported host: $(uname -s)/$(uname -m)" >&2
        exit 1
        ;;
esac

archive="Xray-${platform}-${architecture}.zip"
url="https://github.com/${REPOSITORY}/releases/download/${XRAY_VERSION}/${archive}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

mkdir -p "$ASSETS_DIR/$directory"

echo "Downloading Xray ${XRAY_VERSION} for ${platform}/${architecture}..."
curl --fail --location --silent --show-error "$url" \
    --output "$tmp_dir/xray.zip"

unzip -q "$tmp_dir/xray.zip" -d "$tmp_dir/extracted"

binary_name="xray"
if [[ "$platform" == "windows" ]]; then
    binary_name="xray.exe"
fi

if [[ ! -f "$tmp_dir/extracted/$binary_name" ]]; then
    echo "Expected binary not found in release archive" >&2
    exit 1
fi

cp "$tmp_dir/extracted/$binary_name" "$ASSETS_DIR/$directory/"
chmod +x "$ASSETS_DIR/$directory/$binary_name"

for asset in geoip.dat geosite.dat; do
    if [[ -f "$tmp_dir/extracted/$asset" ]]; then
        cp "$tmp_dir/extracted/$asset" "$ASSETS_DIR/$directory/"
    fi
done

echo "Xray assets prepared in $ASSETS_DIR/$directory"
