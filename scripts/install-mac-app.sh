#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

source_app="${APP_SOURCE:-build/CodeAfar.app}"
destination="${APP_DESTINATION:-/Applications/CodeAfar.app}"
parent="$(dirname "${destination}")"
staging="${parent}/.CodeAfar.installing.$$"
previous="${parent}/.CodeAfar.previous.$$"
had_previous=0
installed=0

cleanup() {
  /bin/rm -rf "${staging}"
  if [[ "${installed}" == "1" ]]; then
    /bin/rm -rf "${previous}"
  fi
}

restore_previous() {
  /usr/bin/pkill -x codeafar >/dev/null 2>&1 || true
  /bin/rm -rf "${destination}"
  if [[ "${had_previous}" == "1" && -d "${previous}" ]]; then
    /bin/mv "${previous}" "${destination}"
  fi
}

trap cleanup EXIT

[[ -x "${source_app}/Contents/MacOS/codeafar" ]] || {
  echo "Invalid CodeAfar bundle: ${source_app}" >&2
  exit 1
}
/usr/bin/codesign --verify --deep --strict "${source_app}"

/bin/rm -rf "${staging}" "${previous}"
/usr/bin/ditto "${source_app}" "${staging}"
/usr/bin/codesign --verify --deep --strict "${staging}"

/usr/bin/pkill -x codeafar >/dev/null 2>&1 || true
for _ in {1..40}; do
  /usr/bin/pgrep -x codeafar >/dev/null 2>&1 || break
  sleep 0.1
done

if [[ -d "${destination}" ]]; then
  /bin/mv "${destination}" "${previous}"
  had_previous=1
fi
/bin/mv "${staging}" "${destination}"

if ! /usr/bin/codesign --verify --deep --strict "${destination}"; then
  restore_previous
  exit 1
fi

open "${destination}"
for _ in {1..60}; do
  if /usr/bin/pgrep -x codeafar >/dev/null 2>&1; then
    installed=1
    echo "Installed and launched ${destination}"
    exit 0
  fi
  sleep 0.25
done

echo "CodeAfar did not launch; restoring previous installation" >&2
restore_previous
exit 1
