import Foundation
import Observation

@MainActor @Observable
final class SessionStore {
    private(set) var sessions: [SessionInfo] = []
    private(set) var projects: [ProjectInfo] = []
    private(set) var templates: [PromptTemplate] = []
    var selectedSessionID: String?
    private let socket: WebSocketClient

    init(socket: WebSocketClient) { self.socket = socket }
    func handle(_ message: ServerMessage) {
        switch message {
        case .hello:
            sendControl("list_sessions", ["limit": 100]); sendControl("list_projects"); sendControl("list_templates")
        case .sessionList(let value): sessions = value
        case .projectList(let value): projects = value
        case .templateList(let value): templates = value
        case .sessionCreated(let id, let name, let cwd):
            selectedSessionID = id
            sessions.append(SessionInfo(sessionId: id, name: name, status: "active", owner: "iPhone", subscribers: [], createdAt: Int64(Date().timeIntervalSince1970)))
            _ = cwd
            sendControl("list_sessions", ["limit": 100])
        case .sessionStopped(let id): sessions.removeAll { $0.sessionId == id }; if selectedSessionID == id { selectedSessionID = nil }
        default: break
        }
    }
    func create(project: ProjectInfo?, permission: String) { sendControl("create_session", ["name": "iPhone 会话", "workingDir": project?.path ?? "", "permissionMode": permission]) }
    func select(_ session: SessionInfo) { selectedSessionID = session.sessionId; sendControl("select_session", ["sessionId": session.sessionId]); sendControl("load_history", ["sessionId": session.sessionId, "limit": 500]) }
    func stop(_ sessionID: String) { sendControl("stop_session", ["sessionId": sessionID]) }
    private func sendControl(_ action: String, _ fields: [String: Any] = [:]) { Task { try? await socket.send(.control(action: action, fields: fields)) } }
}
