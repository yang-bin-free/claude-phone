import Foundation
import NetworkExtension
import Observation

enum TunnelState: Equatable { case disconnected, connecting, connected, failed(String) }

@MainActor protocol TunnelControlling: AnyObject {
    var state: TunnelState { get }
    func start() async throws
    func stop()
}

@MainActor @Observable
final class TunnelController: TunnelControlling {
    private(set) var state: TunnelState = .disconnected
    private var manager: NETunnelProviderManager?

    func start() async throws {
        state = .connecting
        do {
            let manager = try await loadManager()
            try manager.connection.startVPNTunnel()
            self.manager = manager
            state = .connected
        } catch {
            state = .failed(error.localizedDescription)
            throw error
        }
    }

    func stop() { manager?.connection.stopVPNTunnel(); state = .disconnected }

    private func loadManager() async throws -> NETunnelProviderManager {
        let managers = try await NETunnelProviderManager.loadAllFromPreferences()
        let manager = managers.first(where: { ($0.protocolConfiguration as? NETunnelProviderProtocol)?.providerBundleIdentifier == tunnelBundleIdentifier }) ?? NETunnelProviderManager()
        let configuration = NETunnelProviderProtocol()
        configuration.providerBundleIdentifier = tunnelBundleIdentifier
        configuration.serverAddress = "CodeAfar Tailnet"
        manager.protocolConfiguration = configuration
        manager.localizedDescription = "CodeAfar"
        manager.isEnabled = true
        try await manager.saveToPreferences()
        try await manager.loadFromPreferences()
        return manager
    }

    private var tunnelBundleIdentifier: String {
        (Bundle.main.object(forInfoDictionaryKey: "TunnelBundleIdentifier") as? String) ?? "com.example.ClaudePhone.tunnel"
    }
}
