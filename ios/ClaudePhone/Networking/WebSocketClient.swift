import Foundation

struct RetryPolicy {
    private static let seconds = [1, 2, 4, 8, 15]
    static func delay(attempt: Int) -> Duration { .seconds(seconds[min(max(attempt, 0), seconds.count - 1)]) }
}

@MainActor
final class WebSocketClient: NSObject, URLSessionWebSocketDelegate {
    enum State: Equatable { case disconnected, connecting, connected, failed(String) }
    private(set) var state: State = .disconnected
    var onMessage: ((ServerMessage) -> Void)?
    private var session: URLSession!
    private var task: URLSessionWebSocketTask?
    private var retryTask: Task<Void, Never>?
    private var attempt = 0
    private var endpoint: URL?
    private var token = ""
    private let maxMessageBytes = 4 * 1024 * 1024

    override init() { super.init(); session = URLSession(configuration: .ephemeral, delegate: self, delegateQueue: nil) }
    func connect(endpoint: URL, deviceToken: String) {
        self.endpoint = endpoint; token = deviceToken; retryTask?.cancel(); state = .connecting
        let task = session.webSocketTask(with: endpoint); self.task = task; task.resume(); receive()
    }
    func disconnect() { retryTask?.cancel(); task?.cancel(with: .normalClosure, reason: nil); task = nil; state = .disconnected }
    func send(_ message: ClientMessage) async throws { try await task?.send(.data(message.data())) }

    nonisolated func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, didOpenWithProtocol protocol: String?) {
        Task { @MainActor in self.state = .connected; self.attempt = 0; try? await self.send(.auth(deviceToken: self.token, deviceName: "iPhone")) }
    }
    nonisolated func urlSession(_ session: URLSession, webSocketTask: URLSessionWebSocketTask, didCloseWith closeCode: URLSessionWebSocketTask.CloseCode, reason: Data?) { Task { @MainActor in self.scheduleReconnect() } }
    private func receive() {
        task?.receive { [weak self] result in Task { @MainActor in
            guard let self else { return }
            switch result {
            case .success(let message):
                let data: Data = switch message { case .data(let value): value; case .string(let value): Data(value.utf8); @unknown default: Data() }
                if data.count <= self.maxMessageBytes, let decoded = try? JSONDecoder().decode(ServerMessage.self, from: data) { self.onMessage?(decoded) }
                self.receive()
            case .failure: self.scheduleReconnect()
            }
        } }
    }
    private func scheduleReconnect() {
        guard let endpoint else { state = .disconnected; return }
        state = .connecting; let delay = RetryPolicy.delay(attempt: attempt); attempt += 1
        retryTask?.cancel(); retryTask = Task { try? await Task.sleep(for: delay); guard !Task.isCancelled else { return }; connect(endpoint: endpoint, deviceToken: token) }
    }
}
