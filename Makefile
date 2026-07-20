.PHONY: test test-race build-mac build-agent mac-app verify-mac-app mac-release android-aar android-apk ios-framework ios-validate release verify

test:
	go test ./...

test-race:
	go test -race ./pkg/engine

build-mac:
	go build -o build/codeafar ./cmd/mac-app

build-agent:
	go build -o build/codeafar-agent ./cmd/mac-agent

mac-app:
	./scripts/build-mac-app.sh

verify-mac-app:
	@set -eu; \
	app="build/CodeAfar.app"; \
	test -x "$$app/Contents/MacOS/codeafar"; \
	test -f "$$app/Contents/Resources/AppIcon.icns"; \
	for file in LICENSE NOTICE THIRD_PARTY_LICENSES.md; do test -f "$$app/Contents/Resources/$$file"; done; \
	test "$$(plutil -extract CFBundleIdentifier raw "$$app/Contents/Info.plist")" = "com.codeafar.mac"; \
	test "$$(plutil -extract CFBundleIconFile raw "$$app/Contents/Info.plist")" = "AppIcon"; \
	test "$$(plutil -extract LSMinimumSystemVersion raw "$$app/Contents/Info.plist")" = "12.0"; \
	plutil -extract CFBundleShortVersionString raw "$$app/Contents/Info.plist" | grep -Eq '^[0-9A-Za-z][0-9A-Za-z.-]*$$'; \
	if grep -R -a -F "$$(pwd)" "$$app" >/dev/null; then echo "bundle contains workspace path" >&2; exit 1; fi; \
	if codesign -dv "$$app" >/dev/null 2>&1; then codesign --verify --deep --strict "$$app"; fi; \
	echo "Verified $$app"

mac-release:
	MAC_ONLY=1 ./scripts/package-release.sh

android-aar:
	REBUILD_AAR=1 ./scripts/build-android-aar.sh

android-apk:
	cd android && ./build-android.sh clean :app:assembleDebug --no-daemon

ios-framework:
	./scripts/build-ios-framework.sh

ios-validate:
	./scripts/validate-ios-project.sh

release: android-apk
	./scripts/package-release.sh

verify: test test-race build-mac build-agent android-aar
	node --check web/chat/chat.js
	node --check web/admin/admin.js
	git diff --check
	./scripts/test-android-aar.sh
	./scripts/validate-ios-project.sh
