# Claude Phone for iOS

The iOS client targets iOS 18 and mirrors the Android business flow with native SwiftUI:
pairing, embedded Tailscale, sessions, projects, permission modes, templates, history,
streaming, tool calls, queue state, health, reconnect, speech input, settings, and logout.

## Prerequisites

- Full Xcode with the iOS 18 SDK (Command Line Tools alone are insufficient)
- Go and gomobile versions from the repository `go.mod`
- An Apple Developer Team with the Network Extension entitlement
- Registered App IDs for the app and Packet Tunnel Extension
- One App Group enabled for both targets

No Team ID, signing identity, provisioning profile, Device Token, or Tailscale Auth Key is
stored in this repository.

## Configure signing

Open `ios/ClaudePhone.xcodeproj`. Override these values in a local xcconfig or in Xcode:

```text
PRODUCT_BUNDLE_IDENTIFIER = com.yourcompany.ClaudePhone
TUNNEL_BUNDLE_IDENTIFIER = com.yourcompany.ClaudePhone.tunnel
APP_GROUP_IDENTIFIER = group.com.yourcompany.ClaudePhone
DEVELOPMENT_TEAM = YOUR_TEAM_ID
```

Enable the App Group for both targets and Packet Tunnel Provider for `ClaudePhoneTunnel`.
Do not commit the local Team ID override.

## Build

```bash
go install golang.org/x/mobile/cmd/gomobile@v0.0.0-20260611195102-4dd8f1dbf5d2
gomobile init
make ios-framework
xcodebuild build \
  -project ios/ClaudePhone.xcodeproj \
  -scheme ClaudePhone \
  -destination 'generic/platform=iOS'
```

Run source validation without Xcode:

```bash
./scripts/validate-ios-project.sh
```

This validates project structure, iOS 18 settings, plist/entitlements, Swift syntax, shell
scripts, and credential hygiene. On this development machine `/usr/bin/make` is currently
blocked by an unaccepted Xcode license, so the validator is also directly executable.

## Required device verification

After signing is configured, verify on an iOS 18 device:

1. Approve VPN creation and enroll with a one-time Tailscale Auth Key.
2. Confirm the Auth Key is removed after both successful and failed starts.
3. Connect to the Mac over the tailnet with a Mac-generated Device Token.
4. Create and resume a session; verify history, streaming, tools, queue, and health.
5. Verify speech fills the composer without sending automatically.
6. Background/foreground the app, change networks, and verify reconnection.
7. Logout and confirm VPN, Keychain credential, App Group data, and Tailscale state clear.

IPA export and TestFlight additionally require a distribution certificate and provisioning
profiles. The repository can package an externally exported `build/ios/ClaudePhone.ipa`.
