#!/usr/bin/env bash
set -euo pipefail

app_path="${APP_PATH:-/Applications/Claude Phone.app}"
process_name="claude-phone"

menu_click() {
  local item="$1"
  osascript <<APPLESCRIPT
tell application "System Events"
  tell process "${process_name}"
    click menu bar item "CP" of menu bar 1
    delay 0.2
    click menu item "${item}" of menu 1 of menu bar item "CP" of menu bar 1
  end tell
end tell
APPLESCRIPT
}

window_count() {
  osascript -e "tell application \"System Events\" to tell process \"${process_name}\" to count windows"
}

menu_ready() {
  osascript -e "tell application \"System Events\" to tell process \"${process_name}\" to exists menu bar item \"CP\" of menu bar 1" 2>/dev/null
}

open "${app_path}"
for _ in {1..40}; do
  if pgrep -x "${process_name}" >/dev/null && [[ "$(menu_ready)" == "true" ]]; then
    break
  fi
  sleep 0.25
done
if [[ "$(menu_ready)" != "true" ]]; then
  printf 'menu bar item did not become ready\n' >&2
  exit 1
fi

menu_click "打开主窗口"
menu_click "隐藏主窗口"
for _ in {1..20}; do
  if [[ "$(window_count)" == "0" ]]; then
    break
  fi
  sleep 0.1
done
if [[ "$(window_count)" != "0" ]]; then
  printf 'expected the window to be hidden before reopen\n' >&2
  exit 1
fi

open "${app_path}"
for _ in {1..20}; do
  if [[ "$(window_count)" != "0" ]]; then
    exit 0
  fi
  sleep 0.25
done

printf 'reopening the running app did not restore its window\n' >&2
exit 1
