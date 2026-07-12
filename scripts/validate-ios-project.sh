#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cd "${repo_root}"

required=(
  ios/ClaudePhone.xcodeproj/project.pbxproj
  ios/ClaudePhone.xcodeproj/xcshareddata/xcschemes/ClaudePhone.xcscheme
  ios/Config/Base.xcconfig
  ios/ClaudePhone/Info.plist
  ios/ClaudePhone/ClaudePhone.entitlements
  ios/ClaudePhone/ClaudePhoneApp.swift
  ios/ClaudePhoneTunnel/Info.plist
  ios/ClaudePhoneTunnel/ClaudePhoneTunnel.entitlements
  ios/ClaudePhoneTunnel/PacketTunnelProvider.swift
  ios/ClaudePhoneTunnel/TunnelEngineBridge.swift
  ios/ClaudePhoneTunnel/PacketFlowAdapter.swift
  ios/Shared/AppGroup.swift
  ios/Shared/ProtocolModels.swift
  ios/ClaudePhone/Stores/ChatStore.swift
  ios/ClaudePhone/Speech/SpeechController.swift
)
for file in "${required[@]}"; do
  [[ -f "${file}" ]] || { echo "missing iOS project file: ${file}" >&2; exit 1; }
done

for plist in ios/ClaudePhone/Info.plist ios/ClaudePhoneTunnel/Info.plist ios/ClaudePhone/ClaudePhone.entitlements ios/ClaudePhoneTunnel/ClaudePhoneTunnel.entitlements; do
  plutil -lint "${plist}" >/dev/null
done
plutil -lint ios/ClaudePhone.xcodeproj/project.pbxproj >/dev/null

grep -q 'IPHONEOS_DEPLOYMENT_TARGET = 18.0' ios/Config/Base.xcconfig
grep -q 'iOS 18 SDK is required' scripts/build-ios-framework.sh
grep -q 'APP_GROUP_IDENTIFIER' ios/Config/Base.xcconfig
grep -q 'ClaudePhoneTunnel' ios/ClaudePhone.xcodeproj/project.pbxproj
grep -q 'com.apple.networkextension.packet-tunnel' ios/ClaudePhoneTunnel/Info.plist
grep -q '\$(APP_GROUP_IDENTIFIER)' ios/ClaudePhone/ClaudePhone.entitlements
grep -q '\$(APP_GROUP_IDENTIFIER)' ios/ClaudePhoneTunnel/ClaudePhoneTunnel.entitlements

if grep -RInE --exclude='*.md' --exclude='project.pbxproj' '(tskey-auth-[A-Za-z0-9_-]{8,}|dt_[A-Za-z0-9_-]{16,}|DEVELOPMENT_TEAM[[:space:]]*=[[:space:]]*[A-Z0-9]{10})' ios; then
  echo "credential or Team ID detected in iOS sources" >&2
  exit 1
fi

bash -n scripts/build-ios-framework.sh scripts/validate-ios-project.sh
swiftc_bin="$(command -v swiftc || true)"
if [[ -x /Library/Developer/CommandLineTools/usr/bin/swiftc ]]; then
  swiftc_bin=/Library/Developer/CommandLineTools/usr/bin/swiftc
fi
if [[ -n "${swiftc_bin}" ]]; then
  while IFS= read -r source; do "${swiftc_bin}" -parse "${source}"; done < <(find ios -name '*.swift' -type f | sort)
fi
if [[ -x /Library/Developer/CommandLineTools/usr/bin/swiftc ]]; then
  swift_cache="${TMPDIR:-/tmp}/claude-phone-swift-cache"
  mkdir -p "${swift_cache}"
  core_sources="$(find ios/Shared ios/ClaudePhone/Networking ios/ClaudePhone/Stores ios/ClaudePhone/Speech -name '*.swift' -type f | sort)"
  DEVELOPER_DIR=/Library/Developer/CommandLineTools CLANG_MODULE_CACHE_PATH="${swift_cache}" SWIFT_MODULE_CACHE_PATH="${swift_cache}" \
    /usr/bin/xcrun swiftc -typecheck -module-name ClaudePhoneCore ${core_sources}
  tunnel_sources="$(find ios/Shared ios/ClaudePhoneTunnel -name '*.swift' -type f | sort)"
  DEVELOPER_DIR=/Library/Developer/CommandLineTools CLANG_MODULE_CACHE_PATH="${swift_cache}" SWIFT_MODULE_CACHE_PATH="${swift_cache}" \
    /usr/bin/xcrun swiftc -typecheck -module-name ClaudePhoneTunnel ${tunnel_sources}
fi
echo "iOS project structure OK"
