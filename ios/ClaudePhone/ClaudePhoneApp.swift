import SwiftUI

@main
@MainActor
struct ClaudePhoneApp: App {
    @State private var store = AppStore()

    var body: some Scene {
        WindowGroup {
            RootView(store: store)
        }
    }
}
