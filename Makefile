.PHONY: test test-race build-mac build-agent mac-app android-aar android-apk release verify

test:
	go test ./...

test-race:
	go test -race ./pkg/engine

build-mac:
	go build -o build/claude-phone ./cmd/mac-app

build-agent:
	go build -o build/claude-phone-agent ./cmd/mac-agent

mac-app:
	./scripts/build-mac-app.sh

android-aar:
	REBUILD_AAR=1 ./scripts/build-android-aar.sh

android-apk:
	cd android && ./build-android.sh clean :app:assembleDebug --no-daemon

release: mac-app android-apk
	./scripts/package-release.sh

verify: test test-race build-mac build-agent
	node --check web/chat/chat.js
	node --check web/admin/admin.js
	git diff --check
	./scripts/test-android-aar.sh
