#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

aar="${1:-build/claudelib.aar}"
if [[ ! -f "${aar}" ]]; then
  echo "AAR not found: ${aar}" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT
unzip -q "${aar}" classes.jar -d "${tmp_dir}"
jar tf "${tmp_dir}/classes.jar" >"${tmp_dir}/classes.txt"

required_classes=(
  androidlib/Androidlib.class
  tailscale/EngineBackend.class
  tailscale/AppContext.class
  tailscale/IPNService.class
)

for class_name in "${required_classes[@]}"; do
  if ! grep -Fxq "${class_name}" "${tmp_dir}/classes.txt"; then
    echo "missing gomobile class: ${class_name}" >&2
    exit 1
  fi
done

echo "Android AAR contract OK"
