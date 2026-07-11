# Contributing

## Development requirements

- Go version from `go.mod`
- macOS for building the native desktop client
- JDK 17 and Android SDK 35 for the Android app
- `gomobile` for rebuilding `build/claudelib.aar`

## Before opening a pull request

```bash
go test ./...
go test -race ./pkg/engine
node --check web/chat/chat.js
node --check web/admin/admin.js
git diff --check
```

For Android changes:

```bash
REBUILD_AAR=1 ./scripts/build-android-aar.sh
cd android
./build-android.sh clean :app:assembleDebug --no-daemon
```

Never commit Tailscale Auth Keys, device tokens, signing keys, generated APKs,
or generated AAR files. Files derived from Tailscale must preserve their
original copyright and SPDX headers.
