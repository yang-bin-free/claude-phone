import SwiftUI

@main
@MainActor
struct ClaudePhoneApp: App {
    @State private var store = AppStore()
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            RootView(store: store)
                .onChange(of: scenePhase) { _, phase in
                    if phase != .active { Task { await store.speech.stop() } }
                }
        }
    }
}
