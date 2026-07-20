import SwiftUI

struct PairingView: View {
    @Bindable var store: PairingStore
    var body: some View {
        NavigationStack {
            Form {
                Section("连接你的 Mac") {
                    TextField("Mac 地址", text: $store.macAddress).textInputAutocapitalization(.never).autocorrectionDisabled()
                    SecureField("Device Token", text: $store.deviceToken).textContentType(.password)
                    SecureField("Tailscale Auth Key（首次需要）", text: $store.authKey).textContentType(.password)
                    TextField("Control URL（可选）", text: $store.controlURL).textInputAutocapitalization(.never).autocorrectionDisabled()
                }
                if let error = store.errorMessage { Section { Text(error).foregroundStyle(.red) } }
                Button("连接并打开聊天") { Task { await store.connect() } }.buttonStyle(.borderedProminent)
            }.navigationTitle("CodeAfar")
        }
    }
}
