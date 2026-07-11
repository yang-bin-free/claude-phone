import Foundation
import Observation

enum PairingError: LocalizedError {
    case missingAddress, missingDeviceToken, tunnelTimeout
    var errorDescription: String? {
        switch self { case .missingAddress: "请输入 Mac 地址"; case .missingDeviceToken: "请输入 Mac 端生成的 Device Token"; case .tunnelTimeout: "VPN 启动超时，请重试" }
    }
}

@MainActor @Observable
final class PairingStore {
    var macAddress = "claude-mac:9876"
    var authKey = ""
    var deviceToken = ""
    var controlURL = ""
    var errorMessage: String?
    private unowned let app: AppStore
    private let tunnel: TunnelControlling
    private let keychain: KeychainStoring
    private let shared: SharedConfiguring

    init(app: AppStore, tunnel: TunnelControlling, keychain: KeychainStoring, shared: SharedConfiguring) { self.app = app; self.tunnel = tunnel; self.keychain = keychain; self.shared = shared }

    func connect() async {
        do {
            guard !macAddress.trimmingCharacters(in: .whitespaces).isEmpty else { throw PairingError.missingAddress }
            guard !deviceToken.trimmingCharacters(in: .whitespaces).isEmpty else { throw PairingError.missingDeviceToken }
            app.route = .connecting
            try keychain.saveDeviceToken(deviceToken)
            shared.stageAuthKey(authKey)
            shared.set(controlURL, for: SharedKey.controlURL)
            defer { _ = shared.consumeAuthKey(); authKey = "" }
            try await withThrowingTaskGroup(of: Void.self) { group in
                group.addTask { try await self.tunnel.start() }
                group.addTask { try await Task.sleep(for: .seconds(30)); throw PairingError.tunnelTimeout }
                _ = try await group.next(); group.cancelAll()
            }
            app.socket.connect(endpoint: try webSocketURL(macAddress), deviceToken: deviceToken)
            app.route = .chat
        } catch { errorMessage = error.localizedDescription; app.route = .pairing }
    }

    func logout() { tunnel.stop(); app.socket.disconnect(); try? keychain.deleteDeviceToken(); shared.clear(); app.route = .pairing }
    private func webSocketURL(_ value: String) throws -> URL {
        var text = value.trimmingCharacters(in: .whitespacesAndNewlines)
        if !text.contains("://") { text = "ws://" + text }
        if !text.hasSuffix("/ws") { text = text.trimmingCharacters(in: CharacterSet(charactersIn: "/")) + "/ws" }
        guard let url = URL(string: text) else { throw URLError(.badURL) }
        return url
    }
}
