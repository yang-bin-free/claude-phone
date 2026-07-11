import SwiftUI

struct RootView: View {
    @Bindable var store: AppStore
    var body: some View {
        switch store.route {
        case .pairing: PairingView(store: store.pairing)
        case .connecting: ProgressView("正在建立安全隧道…")
        case .chat: SessionListView(app: store)
        }
    }
}
