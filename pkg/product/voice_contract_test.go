package product

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAndroidVoiceUsesOnlyOnDeviceRecognizer(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", ".."))
	controller := readContractFile(t, repo, "android/app/src/main/java/com/claudephone/OnDeviceSpeechController.kt")
	activity := readContractFile(t, repo, "android/app/src/main/java/com/claudephone/MainActivity.kt")
	for _, marker := range []string{"createOnDeviceSpeechRecognizer", "isOnDeviceRecognitionAvailable", "EXTRA_PARTIAL_RESULTS"} {
		if !strings.Contains(controller, marker) {
			t.Errorf("Android on-device controller missing %q", marker)
		}
	}
	for _, forbidden := range []string{"createSpeechRecognizer(", "VOICE_REQUEST_CODE", "launchVoiceRecognition", "startActivityForResult(voiceIntent"} {
		if strings.Contains(controller+activity, forbidden) {
			t.Errorf("Android voice contains cloud/external fallback %q", forbidden)
		}
	}
	if !strings.Contains(activity, "OnDeviceSpeechController") || !strings.Contains(activity, "window.codeAfar.setVoiceText") {
		t.Error("MainActivity does not connect on-device recognition to the CodeAfar composer")
	}
	for _, marker := range []string{"override fun onStop()", "speechController.destroy()", "disconnectAndShowSettings"} {
		if !strings.Contains(activity, marker) {
			t.Errorf("Android voice lifecycle missing %q", marker)
		}
	}
}

func TestIOSVoiceUsesOnlyOnDeviceRecognition(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", ".."))
	controller := readContractFile(t, repo, "ios/ClaudePhone/Speech/SpeechController.swift")
	for _, marker := range []string{
		"requiresOnDeviceRecognition = true",
		"supportsOnDeviceRecognition == true",
		"SpeechAnalyzer",
		"SpeechTranscriber",
		"AssetInventory.assetInstallationRequest",
	} {
		if !strings.Contains(controller, marker) {
			t.Errorf("iOS on-device controller missing %q", marker)
		}
	}
	if strings.Contains(controller, "requiresOnDeviceRecognition = false") {
		t.Error("iOS voice must not fall back to server recognition")
	}
	chat := readContractFile(t, repo, "ios/ClaudePhone/Views/ChatView.swift")
	app := readContractFile(t, repo, "ios/ClaudePhone/ClaudePhoneApp.swift")
	if !strings.Contains(chat, ".onDisappear") || !strings.Contains(app, "scenePhase") {
		t.Error("iOS voice must stop when chat disappears or the app becomes inactive")
	}
}

func readContractFile(t *testing.T, repo, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
