#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

version="${VERSION:-0.1.0-dev}"
if [[ ! "${version}" =~ ^[0-9A-Za-z][0-9A-Za-z.-]*$ ]]; then
  echo "Invalid VERSION: ${version}" >&2
  exit 2
fi

app_dir="build/Claude Phone.app"
contents="${app_dir}/Contents"
rm -rf "${app_dir}"
mkdir -p "${contents}/MacOS" "${contents}/Resources"

icon_work="$(mktemp -d "${TMPDIR:-/tmp}/claude-phone-icon.XXXXXX")"
trap 'rm -rf "${icon_work}"' EXIT
iconset="${icon_work}/AppIcon.iconset"
mkdir -p "${iconset}"
sips -s format png scripts/AppIcon.svg --out "${icon_work}/AppIcon-1024.png" >/dev/null
for spec in \
  "16 icon_16x16.png" "32 icon_16x16@2x.png" \
  "32 icon_32x32.png" "64 icon_32x32@2x.png" \
  "128 icon_128x128.png" "256 icon_128x128@2x.png" \
  "256 icon_256x256.png" "512 icon_256x256@2x.png" \
  "512 icon_512x512.png" "1024 icon_512x512@2x.png"; do
  read -r size name <<<"${spec}"
  sips -z "${size}" "${size}" "${icon_work}/AppIcon-1024.png" --out "${iconset}/${name}" >/dev/null
done
iconutil --convert icns --output "${contents}/Resources/AppIcon.icns" "${iconset}"

go build -trimpath -ldflags "-s -w" -o "${contents}/MacOS/claude-phone" ./cmd/mac-app
cp LICENSE NOTICE THIRD_PARTY_LICENSES.md "${contents}/Resources/"
cp scripts/Info.plist "${contents}/Info.plist"
plutil -replace CFBundleShortVersionString -string "${version}" "${contents}/Info.plist"
plutil -replace CFBundleVersion -string "${version}" "${contents}/Info.plist"

plutil -lint "${contents}/Info.plist"
codesign --force --deep --sign - "${app_dir}"
echo "Built ${app_dir} (${version})"
