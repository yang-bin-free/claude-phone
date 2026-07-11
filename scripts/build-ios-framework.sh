#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cd "${repo_root}"

if ! command -v xcodebuild >/dev/null 2>&1 || ! xcrun --sdk iphoneos --show-sdk-path >/dev/null 2>&1; then
  echo "Full Xcode with the iOS 18 SDK is required; Command Line Tools alone cannot build ClaudeCore.xcframework." >&2
  exit 2
fi
if ! command -v gomobile >/dev/null 2>&1; then
  echo "gomobile is required. Run: go install golang.org/x/mobile/cmd/gomobile@latest" >&2
  exit 2
fi

mkdir -p ios/Frameworks
rm -rf ios/Frameworks/ClaudeCore.xcframework
gomobile bind \
  -target=ios \
  -o ios/Frameworks/ClaudeCore.xcframework \
  github.com/yang-bin-free/claude-phone/pkg/ioslib

echo "Built ios/Frameworks/ClaudeCore.xcframework"
