#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

app_dir="build/Claude Phone.app"
contents="${app_dir}/Contents"
rm -rf "${app_dir}"
mkdir -p "${contents}/MacOS" "${contents}/Resources"

go build -trimpath -ldflags "-s -w" -o "${contents}/MacOS/claude-phone" ./cmd/mac-app
cp LICENSE NOTICE THIRD_PARTY_LICENSES.md "${contents}/Resources/"
cp scripts/Info.plist "${contents}/Info.plist"

plutil -lint "${contents}/Info.plist"
echo "Built ${app_dir}"
