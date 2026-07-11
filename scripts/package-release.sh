#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

version="${VERSION:-0.1.0-dev}"
release_dir="build/release"
mkdir -p "${release_dir}"

./scripts/build-mac-app.sh
ditto -c -k --sequesterRsrc --keepParent "build/Claude Phone.app" "${release_dir}/claude-phone-macos-${version}.zip"

apk="android/app/build/outputs/apk/debug/app-debug.apk"
if [[ -f "${apk}" ]]; then
  cp "${apk}" "${release_dir}/claude-phone-android-${version}.apk"
else
  echo "Android APK not found; run 'make android-apk' before packaging." >&2
fi

cp LICENSE NOTICE THIRD_PARTY_LICENSES.md "${release_dir}/"
rm -f "${release_dir}/SHA256SUMS"
shasum -a 256 "${release_dir}"/* >"${release_dir}/SHA256SUMS"
echo "Release artifacts: ${release_dir}"
