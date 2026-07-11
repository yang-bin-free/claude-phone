# Claude Phone iOS 18 Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a source-complete iOS 18 SwiftUI client with an embedded Packet Tunnel Extension, shared Go/Tailscale XCFramework adapter, native chat/voice flows, tests, CI validation, and build documentation.

**Architecture:** The SwiftUI app owns pairing, Keychain credentials, WebSocket protocol state, sessions, chat, templates, and speech. A Network Extension target owns VPN lifecycle and calls a narrow gomobile-generated Go iOS adapter backed by a packet-flow bridge. App and extension communicate only through an App Group status/config store.

**Tech Stack:** Swift 5.10, SwiftUI, Observation, NetworkExtension, Speech, AVFoundation, Security, URLSessionWebSocketTask, XCTest, Go 1.26, gomobile, Tailscale userspace engine.

## Global Constraints

- Deployment target is exactly iOS 18.0.
- UI is native SwiftUI; do not embed the existing chat WebView.
- Business capabilities and user flow match Android: pairing, embedded tailnet, sessions, projects, permission modes, templates, history, streaming, tools, queue, health, reconnect, voice, settings, and logout.
- The user installs one app; do not depend on the official Tailscale app.
- Bundle identifiers and App Group identifiers are build-setting variables; never commit a Team ID, signing identity, provisioning profile, Auth Key, or Device Token.
- Device Token persists only in Keychain. Tailscale Auth Key is removed from shared storage after every tunnel-start attempt.
- Current-machine success means source/static validation; IPA and device VPN success require full Xcode, iOS 18 SDK, Apple Developer Team, Network Extension entitlement, and a device.

---

## File Map

- `pkg/ioslib/ioslib.go`: gomobile-safe iOS engine facade.
- `pkg/ioslib/packetflow.go`: Go interfaces implemented by Swift Packet Tunnel code.
- `pkg/ioslib/engine_ios.go`: iOS Tailscale engine lifecycle.
- `pkg/ioslib/engine_stub.go`: host-test fallback.
- `scripts/build-ios-framework.sh`: build `ClaudeCore.xcframework` when iPhoneOS SDK exists.
- `scripts/validate-ios-project.sh`: deterministic validation without Xcode.
- `ios/Config/*.xcconfig`: replaceable bundle/App Group/signing settings.
- `ios/ClaudePhone.xcodeproj/project.pbxproj`: app, extension, and test targets.
- `ios/Shared/`: App Group keys, protocol models, Keychain, and shared errors.
- `ios/ClaudePhone/Networking/`: WebSocket and tunnel controllers.
- `ios/ClaudePhone/Stores/`: pairing, session, chat, and app state.
- `ios/ClaudePhone/Views/`: SwiftUI feature views.
- `ios/ClaudePhone/Speech/`: native speech controller.
- `ios/ClaudePhoneTunnel/`: Packet Tunnel provider and Go bridge.
- `ios/ClaudePhoneTests/`: protocol, state, retry, history, and credential tests.

---

### Task 1: iOS Go Adapter and Packet Flow Contract

**Files:**
- Create: `pkg/ioslib/ioslib.go`
- Create: `pkg/ioslib/packetflow.go`
- Create: `pkg/ioslib/engine_ios.go`
- Create: `pkg/ioslib/engine_stub.go`
- Test: `pkg/ioslib/ioslib_test.go`

**Interfaces:**
- Consumes: Tailscale state directory, hostname, Auth Key, Control URL, and a Swift implementation of `PacketFlow`.
- Produces: `Start(dataDir, hostname, authKey, controlURL string, flow PacketFlow) (*Engine, error)`, `(*Engine).Stop()`, `(*Engine).Status() string`; `PacketFlow.Configure(string) error`, `ReadPackets() ([]byte, error)`, `WritePackets([]byte) error`, and `Log(string)`.

- [ ] **Step 1: Write host tests for facade lifecycle**

```go
func TestEngineRejectsMissingPacketFlow(t *testing.T) {
    if _, err := Start(t.TempDir(), "claude-phone-ios", "", "", nil); err == nil {
        t.Fatal("expected missing packet flow to fail")
    }
}
```

- [ ] **Step 2: Run the failing test**

Run: `go test ./pkg/ioslib`

Expected: FAIL because package and `Start` do not exist.

- [ ] **Step 3: Add gomobile-safe contracts and host fallback**

```go
type PacketFlow interface {
    Configure(networkSettingsJSON string) error
    ReadPackets() ([]byte, error)
    WritePackets(packetBatchJSON []byte) error
    Log(line string)
}

type Engine struct { stop func() error; status func() string }
func (e *Engine) Stop() error { return e.stop() }
func (e *Engine) Status() string { return e.status() }
```

The `!ios` fallback validates inputs and returns `ErrIOSOnly`; the `ios` file owns the real Tailscale engine. Keep exported argument and return types gomobile-compatible.

- [ ] **Step 4: Add iOS packet bridge lifecycle**

Create a userspace TUN adapter whose read side obtains JSON-encoded packet batches from Swift and whose write side returns batches to `NEPacketTunnelFlow`. Configure routes/DNS by passing one JSON settings document to Swift. `Stop` must close the Tailscale backend, TUN adapter, and reader goroutine exactly once.

- [ ] **Step 5: Verify host tests and mobile bind package discovery**

Run: `go test ./pkg/ioslib ./pkg/engine ./pkg/session`

Expected: PASS.

Run: `GOOS=ios GOARCH=arm64 go list ./pkg/ioslib`

Expected: package path printed; if iPhoneOS SDK absence blocks cgo, the validation script records that as an external toolchain prerequisite rather than suppressing source checks.

- [ ] **Step 6: Commit**

```bash
git add pkg/ioslib
git commit -m "feat: add iOS Go packet tunnel adapter"
```

### Task 2: XCFramework Build and Static Project Validator

**Files:**
- Create: `scripts/build-ios-framework.sh`
- Create: `scripts/validate-ios-project.sh`
- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `pkg/ioslib`, gomobile, Xcode/iPhoneOS SDK when available.
- Produces: `ios/Frameworks/ClaudeCore.xcframework`; `make ios-validate`; `make ios-framework`.

- [ ] **Step 1: Add a failing validator invocation**

Add Make targets:

```make
ios-validate:
	./scripts/validate-ios-project.sh

ios-framework:
	./scripts/build-ios-framework.sh
```

Run: `make ios-validate`

Expected: FAIL because the validator does not exist.

- [ ] **Step 2: Implement XCFramework build script**

The script must use `set -euo pipefail`, resolve an absolute repository root, require `xcodebuild` with an iPhoneOS SDK, require gomobile, and run:

```bash
gomobile bind -target=ios -o ios/Frameworks/ClaudeCore.xcframework \
  github.com/yang-bin-free/claude-phone/pkg/ioslib
```

Exit with a precise prerequisite message when the active developer directory is Command Line Tools.

- [ ] **Step 3: Implement deterministic static validation**

Validate exact files, target names, iOS 18 deployment settings, App Group variables, both entitlements, required privacy descriptions, extension point identifier, all Swift source references, shell syntax, and absence of credential-shaped strings. Use only `/bin/bash`, `grep`, `plutil`, and `find` available on macOS runners.

- [ ] **Step 4: Add CI job step**

```yaml
- name: Validate iOS source project
  run: make ios-validate
```

- [ ] **Step 5: Run and commit**

Run: `bash -n scripts/build-ios-framework.sh scripts/validate-ios-project.sh`

Run: `make ios-validate`

Expected: FAIL only until Task 3 adds the project, then PASS.

```bash
git add scripts Makefile .github/workflows/ci.yml
git commit -m "build: add iOS framework and project validation"
```

### Task 3: Xcode Project, Configuration, Entitlements, and App Skeleton

**Files:**
- Create: `ios/ClaudePhone.xcodeproj/project.pbxproj`
- Create: `ios/Config/Base.xcconfig`
- Create: `ios/Config/Debug.xcconfig`
- Create: `ios/Config/Release.xcconfig`
- Create: `ios/ClaudePhone/Info.plist`
- Create: `ios/ClaudePhone/ClaudePhone.entitlements`
- Create: `ios/ClaudePhone/ClaudePhoneApp.swift`
- Create: `ios/ClaudePhoneTunnel/Info.plist`
- Create: `ios/ClaudePhoneTunnel/ClaudePhoneTunnel.entitlements`
- Create: `ios/ClaudePhoneTunnel/PacketTunnelProvider.swift`
- Create: `ios/Shared/AppGroup.swift`

**Interfaces:**
- Produces targets `ClaudePhone`, `ClaudePhoneTunnel`, and `ClaudePhoneTests`; build variables `PRODUCT_BUNDLE_IDENTIFIER`, `TUNNEL_BUNDLE_IDENTIFIER`, `APP_GROUP_IDENTIFIER`, and optional `DEVELOPMENT_TEAM`.

- [ ] **Step 1: Create configuration with no committed signing identity**

```xcconfig
IPHONEOS_DEPLOYMENT_TARGET = 18.0
SWIFT_VERSION = 5.0
PRODUCT_BUNDLE_IDENTIFIER = com.example.ClaudePhone
TUNNEL_BUNDLE_IDENTIFIER = com.example.ClaudePhone.tunnel
APP_GROUP_IDENTIFIER = group.com.example.ClaudePhone
CODE_SIGN_STYLE = Automatic
```

- [ ] **Step 2: Add app and extension metadata**

App plist includes `NSSpeechRecognitionUsageDescription` and `NSMicrophoneUsageDescription`. Extension plist includes:

```xml
<key>NSExtension</key>
<dict>
  <key>NSExtensionPointIdentifier</key>
  <string>com.apple.networkextension.packet-tunnel</string>
  <key>NSExtensionPrincipalClass</key>
  <string>$(PRODUCT_MODULE_NAME).PacketTunnelProvider</string>
</dict>
```

Both entitlement files use `$(APP_GROUP_IDENTIFIER)`; the extension additionally enables `packet-tunnel-provider`.

- [ ] **Step 3: Create minimal SwiftUI and tunnel entry points**

`ClaudePhoneApp` instantiates one `AppStore` and renders `RootView`. `PacketTunnelProvider` provides explicit `startTunnel`, `stopTunnel`, and `handleAppMessage` methods and delegates Go work to `TunnelEngineBridge` added in Task 7.

- [ ] **Step 4: Add all targets and source membership to pbxproj**

The app embeds the `.appex`; only the extension links NetworkExtension and ClaudeCore; the test target links the app module. Use stable, unique 24-character PBX identifiers.

- [ ] **Step 5: Validate and commit**

Run: `plutil -lint ios/ClaudePhone/Info.plist ios/ClaudePhoneTunnel/Info.plist ios/ClaudePhone/ClaudePhone.entitlements ios/ClaudePhoneTunnel/ClaudePhoneTunnel.entitlements`

Run: `make ios-validate`

Expected: PASS for structure and metadata.

```bash
git add ios
git commit -m "feat: scaffold iOS app and packet tunnel targets"
```

### Task 4: Shared Protocol, Keychain, and WebSocket Client

**Files:**
- Create: `ios/Shared/ProtocolModels.swift`
- Create: `ios/Shared/KeychainStore.swift`
- Create: `ios/Shared/SharedConfiguration.swift`
- Create: `ios/ClaudePhone/Networking/WebSocketClient.swift`
- Test: `ios/ClaudePhoneTests/ProtocolModelsTests.swift`
- Test: `ios/ClaudePhoneTests/RetryPolicyTests.swift`

**Interfaces:**
- Produces `ClientMessage`, `ServerMessage`, `SessionInfo`, `ProjectInfo`, `PromptTemplate`, `KeychainStoring`, `SharedConfiguring`, `WebSocketTransport`, and `RetryPolicy.delay(attempt:)`.

- [ ] **Step 1: Write Codable fixture tests**

Use fixtures for hello, session list, history, token, tool use, queued, dequeued, health, done, and error. Assert unknown fields decode and unknown message types map to `.unknown(type:)`.

- [ ] **Step 2: Implement protocol models**

Use a custom `ServerMessage.init(from:)` that reads `type` first and dispatches to typed payloads. Encode outbound auth, control, and text with exact camelCase keys used by `pkg/protocol`.

- [ ] **Step 3: Implement Keychain and shared configuration**

`KeychainStore` stores Device Token with `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`. `SharedConfiguration.consumeAuthKey()` returns and deletes the Auth Key in one serialized operation. Logout deletes both stores and the Tailscale state directory.

- [ ] **Step 4: Implement WebSocket transport and retry**

Use `URLSessionWebSocketTask`, cap inbound data at 4 MiB, authenticate immediately after open, and schedule delays `[1, 2, 4, 8, 15]`. A VPN-down callback cancels retry tasks.

- [ ] **Step 5: Run tests when Xcode is available and static checks now**

Run now: `make ios-validate`

Run with Xcode: `xcodebuild test -project ios/ClaudePhone.xcodeproj -scheme ClaudePhone -destination 'platform=iOS Simulator,name=iPhone 16'`

- [ ] **Step 6: Commit**

```bash
git add ios/Shared ios/ClaudePhone/Networking ios/ClaudePhoneTests
git commit -m "feat: add native iOS protocol and WebSocket client"
```

### Task 5: Pairing, Tunnel Management, and App State

**Files:**
- Create: `ios/ClaudePhone/Networking/TunnelController.swift`
- Create: `ios/ClaudePhone/Stores/AppStore.swift`
- Create: `ios/ClaudePhone/Stores/PairingStore.swift`
- Create: `ios/ClaudePhone/Views/RootView.swift`
- Create: `ios/ClaudePhone/Views/PairingView.swift`
- Create: `ios/ClaudePhone/Views/SettingsView.swift`
- Test: `ios/ClaudePhoneTests/PairingStoreTests.swift`

**Interfaces:**
- Produces `TunnelControlling`, `TunnelState`, `PairingStore.connect()`, `PairingStore.logout()`, and root routing states `.pairing`, `.connecting`, `.chat`.

- [ ] **Step 1: Write pairing state tests**

Assert empty Mac address and Device Token fail before VPN requests; successful tunnel start clears Auth Key and transitions to chat; timeout/failure clears Auth Key and exposes a retryable typed error.

- [ ] **Step 2: Implement NETunnelProviderManager controller**

Load or create exactly one manager matching the tunnel bundle identifier, set `NETunnelProviderProtocol.providerBundleIdentifier`, save preferences, reload, and call `startVPNTunnel(options:)` with no secret values in provider configuration.

- [ ] **Step 3: Implement pairing and settings UI**

Pairing fields match Android. Secure fields disable autocorrection and content capture where supported. Settings displays VPN/WebSocket/Mac/protocol/Claude versions and provides stop, re-pair, and destructive logout actions.

- [ ] **Step 4: Validate and commit**

Run: `make ios-validate`

```bash
git add ios/ClaudePhone/Networking/TunnelController.swift ios/ClaudePhone/Stores ios/ClaudePhone/Views
git commit -m "feat: add iOS pairing and tunnel lifecycle"
```

### Task 6: Native Sessions, Chat, Templates, Queue, and Health UI

**Files:**
- Create: `ios/ClaudePhone/Stores/SessionStore.swift`
- Create: `ios/ClaudePhone/Stores/ChatStore.swift`
- Create: `ios/ClaudePhone/Views/SessionListView.swift`
- Create: `ios/ClaudePhone/Views/NewSessionView.swift`
- Create: `ios/ClaudePhone/Views/ChatView.swift`
- Create: `ios/ClaudePhone/Views/MessageRow.swift`
- Create: `ios/ClaudePhone/Views/ToolUseCard.swift`
- Test: `ios/ClaudePhoneTests/ChatStoreTests.swift`

**Interfaces:**
- Consumes typed `ServerMessage` stream.
- Produces native views and state for sessions, projects, templates, history, token batching, tools, queue position, and health.

- [ ] **Step 1: Write store tests**

Verify hello triggers list sessions/projects/templates; session selection sends select then history limit 500; adjacent tokens merge; done closes the active assistant message; queued/dequeued update FIFO badges; stalled/unresponsive health updates the selected session banner.

- [ ] **Step 2: Implement stores**

Keep at most 500 rendered messages per session. Batch token publication to the main actor at one update per display frame using `ContinuousClock` with a 16 ms coalescing window. Preserve complete history models independently of temporary streaming buffers.

- [ ] **Step 3: Implement SwiftUI views**

Use `NavigationStack`, `List`, `ScrollViewReader`, and native sheets. New-session sheet selects project and permission. Template chips fill the composer. Tool and queue rows are visually distinct and accessible.

- [ ] **Step 4: Validate and commit**

Run: `make ios-validate`

```bash
git add ios/ClaudePhone/Stores ios/ClaudePhone/Views ios/ClaudePhoneTests/ChatStoreTests.swift
git commit -m "feat: add native iOS sessions and chat"
```

### Task 7: Speech and Packet Tunnel Go Bridge

**Files:**
- Create: `ios/ClaudePhone/Speech/SpeechController.swift`
- Create: `ios/ClaudePhoneTunnel/TunnelEngineBridge.swift`
- Create: `ios/ClaudePhoneTunnel/PacketFlowAdapter.swift`
- Modify: `ios/ClaudePhoneTunnel/PacketTunnelProvider.swift`
- Test: `ios/ClaudePhoneTests/SpeechControllerTests.swift`

**Interfaces:**
- Produces `SpeechControlling`; Swift implementation of gomobile `IoslibPacketFlow`; tunnel status persistence.

- [ ] **Step 1: Implement permission-testable speech controller**

Inject authorization and audio-engine interfaces. Request Speech and microphone permissions only after tapping the microphone. Return recognized text to the composer without auto-sending.

- [ ] **Step 2: Implement packet flow adapter**

Translate Go settings JSON into `NEPacketTunnelNetworkSettings`, `NEIPv4Settings`, routes, and `NEDNSSettings`. Bridge `readPackets` and `writePackets` using serialized JSON/base64 batches because gomobile cannot export Swift closures or nested byte slices reliably.

- [ ] **Step 3: Implement Extension lifecycle**

Consume and delete Auth Key before calling Go. Set a startup timeout. On every success/failure path update shared status; on failure stop Go and clear secrets; on stop call `Engine.stop()` exactly once.

- [ ] **Step 4: Validate and commit**

Run: `make ios-validate`

Run when Xcode exists: `make ios-framework && xcodebuild build -project ios/ClaudePhone.xcodeproj -scheme ClaudePhone -destination 'generic/platform=iOS' CODE_SIGNING_ALLOWED=NO`

```bash
git add ios/ClaudePhone/Speech ios/ClaudePhoneTunnel ios/ClaudePhoneTests/SpeechControllerTests.swift
git commit -m "feat: connect iOS speech and packet tunnel bridge"
```

### Task 8: Documentation, Release Integration, and Final Verification

**Files:**
- Modify: `README.md`
- Create: `ios/README.md`
- Modify: `scripts/package-release.sh`
- Modify: `docs/superpowers/plans/2026-07-08-project-roadmap.md`

**Interfaces:**
- Produces exact setup/build/signing/troubleshooting instructions and optional IPA packaging when an archive exists.

- [ ] **Step 1: Document prerequisites and configuration**

Document Xcode with iOS 18 SDK, Team selection, App Group registration, Network Extension entitlement request, bundle overrides, XCFramework generation, simulator tests, device tests, and why current Command Line Tools cannot produce an IPA.

- [ ] **Step 2: Add release packaging guard**

If `build/ios/ClaudePhone.ipa` exists, copy it into `build/release/claude-phone-ios-${version}.ipa` before SHA generation. If it does not exist, print one actionable line without failing Mac/Android packaging.

- [ ] **Step 3: Run all available verification**

Run:

```bash
make ios-validate
bash -n scripts/build-ios-framework.sh scripts/validate-ios-project.sh scripts/package-release.sh
go test ./...
go test -race ./pkg/engine ./pkg/session ./pkg/ioslib
node --check web/chat/chat.js
node --check web/admin/admin.js
```

Expected: all commands PASS. `make ios-framework` must fail only with the documented “full Xcode/iPhoneOS SDK required” prerequisite on this machine.

- [ ] **Step 4: Run credential and signing scans**

Search for Tailscale key prefixes, Device Token values, 10-character Apple Team IDs, signing identities, and provisioning profiles. Only documented placeholders and variable names may remain.

- [ ] **Step 5: Commit and push**

```bash
git add README.md ios scripts docs Makefile .github pkg/ioslib
git commit -m "feat: deliver source-complete iOS client"
git push origin master
```

- [ ] **Step 6: Monitor CI**

Wait for Go/security, Android, and iOS validation jobs on the pushed commit. Do not report source delivery complete until all available jobs are green.
