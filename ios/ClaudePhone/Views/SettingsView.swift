import SwiftUI

struct SettingsView: View {
    let app: AppStore
    @Environment(\.dismiss) private var dismiss
    var body: some View {
        NavigationStack {
            Form {
                Section("连接状态") {
                    LabeledContent("VPN", value: String(describing: app.tunnel.state))
                    LabeledContent("WebSocket", value: String(describing: app.socket.state))
                }
                Section { Button("停止 VPN") { app.tunnel.stop() }; Button("退出并清除配置", role: .destructive) { app.pairing.logout(); dismiss() } }
            }.navigationTitle("设置").toolbar { Button("完成") { dismiss() } }
        }
    }
}
