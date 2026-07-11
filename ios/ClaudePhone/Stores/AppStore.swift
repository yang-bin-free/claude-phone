import Foundation
import Observation

@MainActor @Observable
final class AppStore {
    enum Route { case pairing, connecting, chat }
    var route: Route = .pairing
    let tunnel = TunnelController()
    let socket = WebSocketClient()
    let keychain: KeychainStoring
    let shared: SharedConfiguring
    let sessions: SessionStore
    let chat: ChatStore
    let speech: SpeechController
    @ObservationIgnored private var pairingStore: PairingStore?
    var pairing: PairingStore {
        if let pairingStore { return pairingStore }
        let value = PairingStore(app: self, tunnel: tunnel, keychain: keychain, shared: shared)
        pairingStore = value
        return value
    }

    init(keychain: KeychainStoring = KeychainStore(), shared: SharedConfiguring = SharedConfiguration()) {
        self.keychain = keychain; self.shared = shared
        sessions = SessionStore(socket: socket)
        chat = ChatStore(socket: socket)
        speech = SpeechController()
        route = ((try? keychain.deviceToken()) ?? nil) == nil ? .pairing : .chat
        socket.onMessage = { [weak self] message in self?.handle(message) }
        speech.onText = { [weak self] text in self?.chat.composer = text }
    }

    func handle(_ message: ServerMessage) { sessions.handle(message); chat.handle(message) }
}
