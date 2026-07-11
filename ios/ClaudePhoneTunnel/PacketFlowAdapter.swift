import Foundation
import NetworkExtension
#if canImport(ClaudeCore)
import ClaudeCore
#endif

struct PacketBatch: Codable { let packets: [String]; let protocols: [Int32] }
struct GoNetworkSettings: Codable { let mtu: Int; let addresses: [String]?; let routes: [String]?; let dns: [String]? }

final class PacketFlowAdapter: NSObject {
    private let flow: NEPacketTunnelFlow
    private unowned let provider: NEPacketTunnelProvider
    init(flow: NEPacketTunnelFlow, provider: NEPacketTunnelProvider) { self.flow = flow; self.provider = provider }
    func configure(_ json: String?) throws {
        let value = try JSONDecoder().decode(GoNetworkSettings.self, from: Data((json ?? "{}").utf8))
        let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "100.100.100.100"); settings.mtu = NSNumber(value: value.mtu)
        if let address = value.addresses?.first { let ipv4 = NEIPv4Settings(addresses: [address], subnetMasks: ["255.255.255.255"]); ipv4.includedRoutes = (value.routes ?? ["0.0.0.0/0"]).compactMap(Self.route); settings.ipv4Settings = ipv4 }
        if let dns = value.dns, !dns.isEmpty { settings.dnsSettings = NEDNSSettings(servers: dns) }
        let semaphore = DispatchSemaphore(value: 0); var failure: Error?
        provider.setTunnelNetworkSettings(settings) { failure = $0; semaphore.signal() }; semaphore.wait(); if let failure { throw failure }
    }
    func readPackets() throws -> Data {
        let semaphore = DispatchSemaphore(value: 0); var value = Data()
        flow.readPackets { packets, protocols in value = (try? JSONEncoder().encode(PacketBatch(packets: packets.map { $0.base64EncodedString() }, protocols: protocols.map(\.int32Value)))) ?? Data(); semaphore.signal() }
        semaphore.wait(); return value
    }
    func writePackets(_ data: Data?) throws {
        guard let data else { return }; let batch = try JSONDecoder().decode(PacketBatch.self, from: data)
        let packets = batch.packets.compactMap { Data(base64Encoded: $0) }; let protocols = batch.protocols.map(NSNumber.init(value:))
        guard packets.count == protocols.count, flow.writePackets(packets, withProtocols: protocols) else { throw POSIXError(.EIO) }
    }
    func log(_ line: String?) { NSLog("ClaudePhoneTunnel: %@", line ?? "") }
    private static func route(_ cidr: String) -> NEIPv4Route? {
        let parts = cidr.split(separator: "/"); guard parts.count == 2, let bits = Int(parts[1]), bits >= 0, bits <= 32 else { return nil }
        let mask = bits == 0 ? UInt32(0) : UInt32.max << (32 - UInt32(bits)); let text = [24, 16, 8, 0].map { String((mask >> UInt32($0)) & 255) }.joined(separator: ".")
        return NEIPv4Route(destinationAddress: String(parts[0]), subnetMask: text)
    }
}

#if canImport(ClaudeCore)
extension PacketFlowAdapter: IoslibPacketFlow {}
#endif
