import NetworkExtension

final class PacketTunnelProvider: NEPacketTunnelProvider {
    private var engineBridge: TunnelEngineBridge?

    override func startTunnel(options: [String: NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        let bridge = TunnelEngineBridge(packetFlow: packetFlow, provider: self)
        engineBridge = bridge
        bridge.start { result in completionHandler(result.failure) }
    }

    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        engineBridge?.stop()
        engineBridge = nil
        completionHandler()
    }

    override func handleAppMessage(_ messageData: Data, completionHandler: ((Data?) -> Void)? = nil) {
        completionHandler?(engineBridge?.statusData)
    }
}

private extension Result where Success == Void {
    var failure: Failure? { if case .failure(let error) = self { return error }; return nil }
}
