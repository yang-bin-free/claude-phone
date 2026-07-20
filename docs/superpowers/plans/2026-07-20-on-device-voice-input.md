# On-Device Voice Input Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make CodeAfar mobile speech input strictly on-device, editable before send, and explicit when the device or current language cannot support offline recognition.

**Architecture:** Treat speech as a native input method that returns only text to the existing composer. Android owns a `SpeechRecognizer` created exclusively by the on-device factory; iOS selects an iOS 26 `SpeechAnalyzer` adapter or a forced-on-device legacy adapter behind a testable protocol. Neither platform adds an audio transport or cloud fallback.

**Tech Stack:** Kotlin/Android SpeechRecognizer API 31+, Swift 6/SwiftUI, SpeechAnalyzer on iOS 26+, SFSpeechRecognizer on iOS 18–25, AVAudioEngine, WebView JavaScript bridge, XCTest/JUnit and Go contract tests.

## Global Constraints

- Android uses only `SpeechRecognizer.createOnDeviceSpeechRecognizer()` and only when API is at least 31 and `isOnDeviceRecognitionAvailable(context)` is true.
- Android must never call `createSpeechRecognizer()` or launch `ACTION_RECOGNIZE_SPEECH`.
- iOS 26+ uses `SpeechAnalyzer` and `SpeechTranscriber`.
- iOS 18–25 requires `supportsOnDeviceRecognition == true` and sets `requiresOnDeviceRecognition = true`.
- There is no cloud fallback; unsupported device/language states are shown to the user.
- Recognition text fills or appends to the editable composer and never auto-sends.
- Audio is never uploaded to the Mac, a provider, or a third-party transcription service, and is never saved to disk.

---

## File Structure

- `web/chat/chat.js`, `web/chat/index.html`, `web/chat/mobile.css`: shared append semantics and Android voice status.
- `android/.../OnDeviceSpeechController.kt`: Android API gate, lifecycle, recognition callbacks, error mapping.
- `android/.../MainActivity.kt`: permission and WebView bridge only.
- `ios/ClaudePhone/Speech/OnDeviceSpeechEngine.swift`: testable engine protocol and state/result types.
- `ios/ClaudePhone/Speech/LegacyOnDeviceSpeechEngine.swift`: forced local iOS 18–25 path.
- `ios/ClaudePhone/Speech/SpeechAnalyzerEngine.swift`: iOS 26+ path.
- `ios/ClaudePhone/Speech/SpeechController.swift`: permissions, state machine, adapter selection, cleanup.
- Platform tests validate state transitions without opening a real microphone.

### Task 1: Shared Composer Voice Contract

**Files:**
- Modify: `web/chat/index.html`
- Modify: `web/chat/chat.js`
- Modify: `web/chat/mobile.css`
- Modify: `web/design_regression_test.go`

**Interfaces:**
- Produces: `window.codeAfar.setVoiceText(text, final)` and `window.codeAfar.setVoiceState(state, message)`.
- Preserves: user text existing before recording; live partials replace only the active voice segment.

- [ ] **Step 1: Add Web contract tests**

```go
func TestVoiceBridgeUpdatesDraftWithoutSubmitting(t *testing.T) {
    // Assert setVoiceText/setVoiceState exist and neither calls send nor form.requestSubmit.
}
func TestVoiceTextAppendsToExistingComposer(t *testing.T) {
    // Assert recording baseline and a natural separator are retained.
}
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./web -run Voice -v`

Expected: FAIL because the current bridge only replaces the prompt.

- [ ] **Step 3: Implement baseline-plus-live-segment behavior**

```js
function startVoiceDraft() {
  state.voiceBase = prompt.value;
}
function setVoiceText(text) {
  const separator = state.voiceBase && text ? (/\s$/.test(state.voiceBase) ? "" : " ") : "";
  prompt.value = `${state.voiceBase}${separator}${text}`;
  prompt.dispatchEvent(new Event("input", { bubbles: true }));
}
function setVoiceState(next, message = "") {
  state.voiceState = next;
  voiceButton.dataset.state = next;
  voiceButton.setAttribute("aria-label", message || (next === "listening" ? "停止语音输入" : "开始语音输入"));
}
```

- [ ] **Step 4: Run Web tests and syntax checks**

Run: `go test ./web && node --check web/chat/chat.js`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web
git commit -m "feat(web): add editable voice draft contract"
```

### Task 2: Android Strict On-Device Recognition

**Files:**
- Create: `android/app/src/main/java/com/claudephone/OnDeviceSpeechController.kt`
- Create: `android/app/src/test/java/com/claudephone/OnDeviceSpeechControllerTest.kt`
- Modify: `android/app/src/main/java/com/claudephone/MainActivity.kt`
- Modify: `android/app/build.gradle.kts`

**Interfaces:**
- Produces: `VoiceState` sealed type and `OnDeviceSpeechController(context, sdkInt, callbacks)`.
- Produces: `toggle(locale: Locale)`, `stop()`, and `destroy()` methods called only on the main thread.
- Consumes: Web functions from Task 1 through `evaluateJavascript`.

- [ ] **Step 1: Add pure API-gate and error-map tests**

```kotlin
@Test fun api30IsUnavailableWithoutCreatingRecognizer() {
    val gate = OnDeviceSpeechGate(30, { true })
    assertEquals(Availability.UnsupportedVersion, gate.availability())
}

@Test fun unavailableServiceNeverFallsBackToCloud() {
    val gate = OnDeviceSpeechGate(35, { false })
    assertEquals(Availability.Unavailable, gate.availability())
}

@Test fun networkErrorsMapToOfflineFailure() {
    assertEquals("端侧识别失败，未使用网络识别", speechErrorMessage(SpeechRecognizer.ERROR_NETWORK))
}
```

- [ ] **Step 2: Verify Android unit tests fail**

Run: `cd android && ./gradlew :app:testDebugUnitTest --no-daemon`

Expected: FAIL because controller/gate types do not exist.

- [ ] **Step 3: Implement on-device creation and callbacks**

```kotlin
if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S ||
    !SpeechRecognizer.isOnDeviceRecognitionAvailable(context)) {
    callbacks.onState(VoiceState.Unavailable("当前设备或语言不支持离线语音输入")); return
}
recognizer = SpeechRecognizer.createOnDeviceSpeechRecognizer(context).also {
    it.setRecognitionListener(listener)
    it.startListening(Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
        putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
        putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true)
        putExtra(RecognizerIntent.EXTRA_LANGUAGE, locale.toLanguageTag())
    })
}
```

Remove `startActivityForResult` voice handling and `VOICE_REQUEST_CODE`. Activity requests microphone permission, calls `toggle`, forwards partial/final text and states to `window.codeAfar`, and calls `destroy()` in `onDestroy`.

- [ ] **Step 4: Prove no network recognizer/fallback remains**

Run: `cd android && ./gradlew :app:testDebugUnitTest :app:assembleDebug --no-daemon && ! rg "createSpeechRecognizer\(|ACTION_RECOGNIZE_SPEECH.*startActivity" app/src/main`

Expected: tests/build PASS and the negative search succeeds.

- [ ] **Step 5: Commit**

```bash
git add android
git commit -m "feat(android): require on-device speech recognition"
```

### Task 3: iOS Testable Controller and Forced-On-Device Legacy Engine

**Files:**
- Create: `ios/ClaudePhone/Speech/OnDeviceSpeechEngine.swift`
- Create: `ios/ClaudePhone/Speech/LegacyOnDeviceSpeechEngine.swift`
- Modify: `ios/ClaudePhone/Speech/SpeechController.swift`
- Modify: `ios/ClaudePhone/Stores/AppStore.swift`
- Modify: `ios/ClaudePhone/Views/ChatView.swift`
- Modify: `ios/ClaudePhoneTests/SpeechControllerTests.swift`
- Modify: `ios/project.yml`

**Interfaces:**
- Produces: `protocol OnDeviceSpeechEngine { var isAvailable: Bool { get }; func start(onText:onFinish:) async throws; func stop() async }`.
- Produces: controller states `idle`, `requestingPermission`, `preparing`, `listening`, `unavailable(String)`, `denied`, `failed(String)`.
- Produces: injected `SpeechController(engineFactory: @escaping () -> any OnDeviceSpeechEngine)` for tests.

- [ ] **Step 1: Write fake-engine controller tests**

```swift
func testPartialTextIsEditableAndNeverSends() async {
    let fake = FakeOnDeviceSpeechEngine(available: true)
    let controller = await SpeechController(engineFactory: { fake })
    var values: [String] = []
    await MainActor.run { controller.onText = { values.append($0) } }
    await controller.start()
    fake.emit("你好")
    XCTAssertEqual(values, ["你好"])
}

func testUnavailableEngineDoesNotStart() async {
    let fake = FakeOnDeviceSpeechEngine(available: false)
    let controller = await SpeechController(engineFactory: { fake })
    await controller.start()
    XCTAssertEqual(await controller.state, .unavailable("当前设备或语言不支持离线语音输入"))
    XCTAssertEqual(fake.startCount, 0)
}
func testStopCleansUpTheActiveEngine() async {
    let fake = FakeOnDeviceSpeechEngine(available: true)
    let controller = await SpeechController(engineFactory: { fake })
    await controller.start()
    await controller.stop()
    XCTAssertEqual(fake.stopCount, 1)
    XCTAssertEqual(await controller.state, .idle)
}
```

- [ ] **Step 2: Verify XCTest fails**

Run: `xcodebuild test -project ios/ClaudePhone.xcodeproj -scheme ClaudePhone -destination 'platform=iOS Simulator,name=iPhone 16'`

Expected: FAIL because the injectable engine protocol is absent.

- [ ] **Step 3: Implement the protocol, controller state machine, and iOS 18–25 adapter**

```swift
guard let recognizer, recognizer.supportsOnDeviceRecognition else {
    throw SpeechFailure.unavailable("当前设备或语言不支持离线语音输入")
}
let request = SFSpeechAudioBufferRecognitionRequest()
request.requiresOnDeviceRecognition = true
request.shouldReportPartialResults = true
```

The controller requests speech and microphone permission before constructing an engine. The legacy engine owns its audio tap/task, never retries without `requiresOnDeviceRecognition`, and removes the tap/deactivates the audio session on stop. The view shows state text and toggles start/stop. `AppStore` appends the voice segment to the composer and does not call `send()`.

- [ ] **Step 4: Run iOS tests and structural validation**

Run: `./scripts/validate-ios-project.sh && xcodebuild test -project ios/ClaudePhone.xcodeproj -scheme ClaudePhone -destination 'platform=iOS Simulator,name=iPhone 16'`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ios
git commit -m "feat(ios): force on-device legacy speech recognition"
```

### Task 4: iOS 26 SpeechAnalyzer Adapter

**Files:**
- Create: `ios/ClaudePhone/Speech/SpeechAnalyzerEngine.swift`
- Create: `ios/ClaudePhoneTests/SpeechAnalyzerEngineTests.swift`
- Modify: `ios/ClaudePhone/Speech/SpeechController.swift`
- Modify: `ios/project.yml`

**Interfaces:**
- Consumes: `OnDeviceSpeechEngine` from Task 3.
- Produces: `@available(iOS 26.0, *) SpeechAnalyzerEngine` and runtime factory selection.

- [ ] **Step 1: Add availability-routing tests**

```swift
func testFactorySelectsLegacyBeforeIOS26() {
    XCTAssertEqual(SpeechEngineKind.forMajorVersion(25), .legacyOnDevice)
}
func testFactorySelectsAnalyzerOnIOS26() {
    XCTAssertEqual(SpeechEngineKind.forMajorVersion(26), .speechAnalyzer)
}
```

- [ ] **Step 2: Verify routing tests fail**

Run: `xcodebuild test -project ios/ClaudePhone.xcodeproj -scheme ClaudePhone -destination 'platform=iOS Simulator,name=iPhone 16' -only-testing:ClaudePhoneTests/SpeechAnalyzerEngineTests`

Expected: FAIL because the analyzer engine kind does not exist.

- [ ] **Step 3: Implement the availability-guarded analyzer**

Use `SpeechTranscriber` with the current locale, check supported/reserved locales, use `AssetInventory` to install a missing Apple-managed local language asset, feed `AVAudioEngine` buffers to `SpeechAnalyzer`, and consume asynchronous transcription results. End the input stream and release the tap, tasks, and audio session on every final/error/stop path. Compile all analyzer references inside `@available(iOS 26.0, *)` declarations and select them only through `if #available(iOS 26.0, *)`.

- [ ] **Step 4: Compile both deployment paths**

Run: `./scripts/validate-ios-project.sh && xcodebuild build -project ios/ClaudePhone.xcodeproj -scheme ClaudePhone -destination 'generic/platform=iOS Simulator' CODE_SIGNING_ALLOWED=NO`

Expected: PASS with the project deployment target still at iOS 18.

- [ ] **Step 5: Commit**

```bash
git add ios
git commit -m "feat(ios): use SpeechAnalyzer on iOS 26"
```

### Task 5: Voice Privacy and Release Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/TESTING.md`
- Create: `scripts/test-voice-contract.sh`

**Interfaces:**
- Produces: a static privacy gate plus platform build/test commands suitable for CI.

- [ ] **Step 1: Write the privacy gate**

```bash
#!/usr/bin/env bash
set -euo pipefail
! rg "createSpeechRecognizer\(" android/app/src/main
! rg "startActivityForResult\(voiceIntent|VOICE_REQUEST_CODE" android/app/src/main
rg "createOnDeviceSpeechRecognizer" android/app/src/main
rg "requiresOnDeviceRecognition = true" ios/ClaudePhone/Speech
! rg "audio/|multipart|upload.*audio" ios/ClaudePhone android/app/src/main web/chat
```

- [ ] **Step 2: Run the gate before final documentation**

Run: `bash scripts/test-voice-contract.sh`

Expected: PASS only after Tasks 2–4 are complete.

- [ ] **Step 3: Document supported versions and explicit limitations**

Document API 31+, iOS 26 analyzer behavior, iOS 18–25 forced local fallback, local language-asset requirements, editable/no-auto-send behavior, and the exact unsupported message. Do not claim physical-device validation unless a device was actually connected and tested offline.

- [ ] **Step 4: Run the full verification suite**

Run: `make verify && (cd android && ./gradlew :app:testDebugUnitTest :app:assembleDebug --no-daemon) && ./scripts/validate-ios-project.sh && bash scripts/test-voice-contract.sh`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/TESTING.md scripts/test-voice-contract.sh
git commit -m "docs: document private on-device voice input"
```

## Final Verification

- [ ] Run `adb devices` and `xcrun xctrace list devices`; record whether real hardware is available.
- [ ] On available Android 12+ hardware, enable airplane mode and transcribe `你好`; verify text appears but is not sent.
- [ ] On available iOS 18–25 or 26+ hardware, disable networking and perform the same check.
- [ ] Deny microphone/speech permission, background the app, double-tap the microphone, and leave the chat; verify clear state and no retained microphone session.
- [ ] If hardware is unavailable, report that boundary explicitly and rely on fake-engine tests, compile checks, static privacy gates, and simulator lifecycle checks.
