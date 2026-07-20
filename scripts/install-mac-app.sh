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
legacy_app="/Applications/Claude Phone.app"
legacy_plist="${HOME}/Library/LaunchAgents/com.claude.phone.plist"
legacy_app_backup="${parent}/.Claude Phone.previous.$$"
legacy_plist_backup="${legacy_plist}.codeafar-backup.$$"
new_plist="${HOME}/Library/LaunchAgents/com.codeafar.mac.plist"
legacy_autostart=0
new_autostart_preexisting=0
migrated_autostart=0
[[ -f "${legacy_plist}" ]] && legacy_autostart=1
[[ -f "${new_plist}" ]] && new_autostart_preexisting=1

installed_pid() {
  /bin/ps -axo pid=,command= | /usr/bin/awk -v executable="${destination}/Contents/MacOS/codeafar" \
    '$2 == executable && !found { print $1; found = 1 }'
}

cleanup() {
  /bin/rm -rf "${staging}"
  if [[ "${installed}" == "1" ]]; then
	/bin/rm -rf "${previous}" "${legacy_app_backup}"
	/bin/rm -f "${legacy_plist_backup}"
  fi
}

restore_previous() {
  /usr/bin/pkill -x codeafar >/dev/null 2>&1 || true
  /bin/rm -rf "${destination}"
  if [[ "${had_previous}" == "1" && -d "${previous}" ]]; then
    /bin/mv "${previous}" "${destination}"
  fi
	if [[ -d "${legacy_app_backup}" ]]; then
	  /bin/mv "${legacy_app_backup}" "${legacy_app}"
	fi
	if [[ -f "${legacy_plist_backup}" ]]; then
	  /bin/mv "${legacy_plist_backup}" "${legacy_plist}"
	fi
	if [[ "${migrated_autostart}" == "1" && "${new_autostart_preexisting}" == "0" ]]; then
	  /bin/launchctl bootout "gui/${UID}/com.codeafar.mac" >/dev/null 2>&1 || true
	  /bin/rm -f "${new_plist}"
	fi
	if [[ "${legacy_autostart}" == "1" && -f "${legacy_plist}" ]]; then
	  /bin/launchctl bootstrap "gui/${UID}" "${legacy_plist}" >/dev/null 2>&1 || true
	fi
}

retire_legacy_installation() {
  if [[ "${legacy_autostart}" == "1" ]]; then
	  migrated_autostart=1
    if ! "${destination}/Contents/MacOS/codeafar" autostart install; then
      echo "Could not migrate the legacy login item" >&2
      return 1
    fi
  fi
  /bin/launchctl bootout "gui/${UID}/com.claude.phone" >/dev/null 2>&1 || true
	if [[ -f "${legacy_plist}" ]] && ! /bin/mv "${legacy_plist}" "${legacy_plist_backup}"; then return 1; fi
	if [[ -d "${legacy_app}" ]] && ! /bin/mv "${legacy_app}" "${legacy_app_backup}"; then return 1; fi
}

trap cleanup EXIT

[[ -x "${source_app}/Contents/MacOS/codeafar" ]] || {
  echo "Invalid CodeAfar bundle: ${source_app}" >&2
  exit 1
}
/usr/bin/codesign --verify --deep --strict "${source_app}"

/bin/rm -rf "${staging}" "${previous}" "${legacy_app_backup}"
/bin/rm -f "${legacy_plist_backup}"
/usr/bin/ditto "${source_app}" "${staging}"
/usr/bin/codesign --verify --deep --strict "${staging}"

/usr/bin/pkill -x codeafar >/dev/null 2>&1 || true
/usr/bin/pkill -x claude-phone >/dev/null 2>&1 || true
for _ in {1..40}; do
  if [[ -z "$(installed_pid)" ]] && ! /usr/sbin/lsof -nP -iTCP:9877 -sTCP:LISTEN >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
if /usr/sbin/lsof -nP -iTCP:9877 -sTCP:LISTEN >/dev/null 2>&1; then
  echo "Port 9877 is still occupied after stopping the previous CodeAfar version" >&2
  exit 1
fi

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
  pid="$(installed_pid)"
  if [[ -n "${pid}" ]] && /usr/sbin/lsof -nP -a -p "${pid}" -iTCP:9877 -sTCP:LISTEN >/dev/null 2>&1; then
    status="$(/usr/bin/curl --silent --fail --max-time 1 http://127.0.0.1:9877/desktop/status 2>/dev/null || true)"
    if [[ "${status}" != *'"ready":true'* ]] || ! /bin/kill -0 "${pid}" >/dev/null 2>&1; then
      continue
    fi
    if ! retire_legacy_installation; then
      restore_previous
      exit 1
    fi
    installed=1
    echo "Installed and launched ${destination} (pid ${pid})"
    exit 0
  fi
  sleep 0.25
done

echo "CodeAfar did not launch; restoring previous installation" >&2
restore_previous
exit 1
