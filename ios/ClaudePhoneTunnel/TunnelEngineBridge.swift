import Foundation
import NetworkExtension
#if canImport(ClaudeCore)
import ClaudeCore
#endif

enum TunnelBridgeError: LocalizedError, Equatable {
    case frameworkMissing, appGroupUnavailable
    var errorDescription: String? { self == .frameworkMissing ? "ClaudeCore.xcframework is missing" : "App Group is unavailable" }
}

final class TunnelEngineBridge {
    private let packetFlow: NEPacketTunnelFlow
    private unowned let provider: NEPacketTunnelProvider
    private let configuration = SharedConfiguration()
#if canImport(ClaudeCore)
    private var engine: IoslibEngine?
#else
    private var engine: AnyObject?
#endif
    private(set) var state = "stopped"
    init(packetFlow: NEPacketTunnelFlow, provider: NEPacketTunnelProvider) { self.packetFlow = packetFlow; self.provider = provider }
    var statusData: Data? { try? JSONSerialization.data(withJSONObject: ["state": state, "error": configuration.string(for: SharedKey.tunnelError) ?? ""]) }
    func start(completion: @escaping (Result<Void, Error>) -> Void) {
        let authKey = configuration.consumeAuthKey() ?? ""; let controlURL = configuration.string(for: SharedKey.controlURL) ?? ""
        configuration.set(nil, for: SharedKey.tunnelError); state = "starting"
        do {
            guard let root = FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: AppGroup.identifier) else { throw TunnelBridgeError.appGroupUnavailable }
            let directory = root.appending(path: "tailscale", directoryHint: .isDirectory); try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true); configuration.set(directory.path, for: SharedKey.stateDirectory)
#if canImport(ClaudeCore)
            let adapter = PacketFlowAdapter(flow: packetFlow, provider: provider); var error: NSError?
            engine = IoslibStart(directory.path, "claude-phone-ios", authKey, controlURL, adapter, &error)
            if let error { throw error }
            state = "connected"; configuration.set(state, for: SharedKey.tunnelState); completion(.success(()))
#else
            throw TunnelBridgeError.frameworkMissing
#endif
        } catch { state = "failed"; configuration.set(state, for: SharedKey.tunnelState); configuration.set(error.localizedDescription, for: SharedKey.tunnelError); completion(.failure(error)) }
    }
    func stop() {
#if canImport(ClaudeCore)
        try? engine?.stop()
#endif
        engine = nil; state = "stopped"; configuration.set(state, for: SharedKey.tunnelState); _ = configuration.consumeAuthKey()
    }
}
