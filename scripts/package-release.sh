#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

version="${VERSION:-0.1.0-dev}"
release_dir="build/release"
mkdir -p "${release_dir}"

VERSION="${version}" ./scripts/build-mac-app.sh
make verify-mac-app
ditto -c -k --sequesterRsrc --keepParent "build/Claude Phone.app" "${release_dir}/claude-phone-macos-${version}.zip"

if [[ "${MAC_ONLY:-0}" != "1" ]]; then
  apk="android/app/build/outputs/apk/debug/app-debug.apk"
  if [[ -f "${apk}" ]]; then
    cp "${apk}" "${release_dir}/claude-phone-android-${version}.apk"
  else
    echo "Android APK not found; run 'make android-apk' before packaging." >&2
  fi

  ipa="build/ios/ClaudePhone.ipa"
  if [[ -f "${ipa}" ]]; then
    cp "${ipa}" "${release_dir}/claude-phone-ios-${version}.ipa"
  else
    echo "iOS IPA not found; export build/ios/ClaudePhone.ipa from signed Xcode archive to include it." >&2
  fi
fi

cp LICENSE NOTICE THIRD_PARTY_LICENSES.md "${release_dir}/"
rm -f "${release_dir}/SHA256SUMS"
shasum -a 256 "${release_dir}"/* >"${release_dir}/SHA256SUMS"
echo "Release artifacts: ${release_dir}"
