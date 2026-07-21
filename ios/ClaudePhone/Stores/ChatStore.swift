import Foundation
import Observation

enum ChatRole: String { case user, assistant, tool, status, error }
struct ChatMessage: Identifiable {
    let id: UUID
    let role: ChatRole
    var text: String
    var queueID: String?
    init(id: UUID = UUID(), role: ChatRole, text: String, queueID: String? = nil) { self.id = id; self.role = role; self.text = text; self.queueID = queueID }
}

@MainActor @Observable
final class ChatStore {
    private(set) var messages: [ChatMessage] = []
    private(set) var healthState = "healthy"
    var composer = ""
    private let socket: WebSocketClient
    private var assistantID: UUID?
    private var pendingTokens = ""
    private var flushTask: Task<Void, Never>?

    init(socket: WebSocketClient) { self.socket = socket }
    func handle(_ message: ServerMessage) {
        switch message {
        case .history(_, let items):
            flushTask?.cancel(); pendingTokens = ""; assistantID = nil; messages = []
            for item in items { replay(item) }
        case .thinking: finishAssistant()
        case .token(let value): queueToken(value)
        case .toolUse: break
        case .queued(let id, let position): append(.status, "已排队（第 \(position) 条）", queueID: id)
        case .dequeued(let id): messages.removeAll { $0.queueID == id }
        case .health(_, let state, let idle): healthState = state; if state != "healthy" { append(.status, state == "stalled" ? "会话可能卡住（\(idle)s）" : "会话无响应（\(idle)s）") }
        case .done: flushTokens(); finishAssistant()
        case .error(let code, let text): append(.error, "\(code): \(text)")
        default: break
        }
    }
    func send() {
        let content = composer.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !content.isEmpty else { return }
        append(.user, content); composer = ""; finishAssistant()
        Task { try? await socket.send(.text(content)) }
    }
    private func queueToken(_ value: String) {
        pendingTokens += value
        guard flushTask == nil else { return }
        flushTask = Task { try? await Task.sleep(for: .milliseconds(16)); guard !Task.isCancelled else { return }; self.flushTokens() }
    }
    private func flushTokens() {
        flushTask?.cancel(); flushTask = nil; guard !pendingTokens.isEmpty else { return }
        if let id = assistantID, let index = messages.firstIndex(where: { $0.id == id }) { messages[index].text += pendingTokens }
        else { let item = ChatMessage(role: .assistant, text: pendingTokens); assistantID = item.id; messages.append(item) }
        pendingTokens = ""; trim()
    }
    private func finishAssistant() { assistantID = nil }
    private func append(_ role: ChatRole, _ text: String, queueID: String? = nil) { messages.append(ChatMessage(role: role, text: text, queueID: queueID)); trim() }
    private func trim() { if messages.count > 500 { messages.removeFirst(messages.count - 500) } }
    private func replay(_ item: HistoryItem) {
        switch item.type { case "text": append(.user, item.content ?? ""); case "token": queueToken(item.content ?? ""); case "tool_use": break; case "done": flushTokens(); finishAssistant(); default: break }
    }
}
