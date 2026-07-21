import Foundation
import Observation

@MainActor @Observable
final class SessionStore {
    private(set) var sessions: [SessionInfo] = []
    private(set) var projects: [ProjectInfo] = []
    private(set) var providers: [ProviderInfo] = []
    private(set) var templates: [PromptTemplate] = []
    var selectedSessionID: String?
    private(set) var activeProvider: String
    private let socket: WebSocketClient
    private let defaults: UserDefaults
    private var lastSessions: [String: String]
    private let activeProviderKey = "CodeAfar.ActiveProvider"
    private let lastSessionsKey = "CodeAfar.LastProviderSessions"

    init(socket: WebSocketClient, defaults: UserDefaults = .standard) {
        self.socket = socket
        self.defaults = defaults
        activeProvider = defaults.string(forKey: activeProviderKey) ?? "claude"
        lastSessions = defaults.dictionary(forKey: lastSessionsKey) as? [String: String] ?? [:]
    }

    var visibleSessions: [SessionInfo] { sessions.filter { $0.provider == activeProvider } }
    var activeProviderInfo: ProviderInfo? { providers.first { $0.id == activeProvider } }

    func handle(_ message: ServerMessage) {
        switch message {
        case .hello:
            sendControl("list_sessions", ["limit": 100]); sendControl("list_projects"); sendControl("list_providers"); sendControl("list_templates")
        case .sessionList(let value):
            sessions = value
            lastSessions = lastSessions.filter { provider, sessionID in
                sessions.contains { $0.provider == provider && $0.sessionId == sessionID }
            }
            if let selectedSessionID, !sessions.contains(where: { $0.sessionId == selectedSessionID }) { self.selectedSessionID = nil }
            restoreSelection()
            persistWorkspace()
        case .projectList(let value): projects = value
        case .providerList(let value):
            providers = value
            normalizeActiveProvider()
            restoreSelection()
        case .templateList(let value): templates = value
        case .sessionCreated(let id, let name, let cwd, let provider, let model, let permissionMode):
            activeProvider = provider
            selectedSessionID = id
            lastSessions[provider] = id
            sessions.removeAll { $0.sessionId == id }
            sessions.append(SessionInfo(sessionId: id, name: name, status: "active", owner: "iPhone", subscribers: [], createdAt: Int64(Date().timeIntervalSince1970), cwd: cwd, provider: provider, model: model, permissionMode: permissionMode))
            persistWorkspace()
            sendControl("list_sessions", ["limit": 100])
        case .sessionStopped(let id):
            sessions.removeAll { $0.sessionId == id }
            lastSessions = lastSessions.filter { $0.value != id }
            if selectedSessionID == id { selectedSessionID = nil }
            persistWorkspace()
        default: break
        }
    }
    func create(project: ProjectInfo?, permission: String) { sendControl("create_session", ["name": "iPhone 会话", "workingDir": project?.path ?? "", "provider": activeProvider, "permissionMode": permission]) }
    func select(_ session: SessionInfo) {
        activeProvider = session.provider
        selectedSessionID = session.sessionId
        lastSessions[session.provider] = session.sessionId
        persistWorkspace()
        sendSelection(session.sessionId)
    }
    func switchProvider(_ id: String) {
        guard providers.first(where: { $0.id == id })?.available == true else { return }
        if let selected = sessions.first(where: { $0.sessionId == selectedSessionID }) {
            lastSessions[selected.provider] = selected.sessionId
        }
        activeProvider = id
        selectedSessionID = sessions.first { $0.provider == id && $0.sessionId == lastSessions[id] }?.sessionId
        persistWorkspace()
        if let selectedSessionID { sendSelection(selectedSessionID) }
    }
    func stop(_ sessionID: String) { sendControl("stop_session", ["sessionId": sessionID]) }
    private func normalizeActiveProvider() {
        if providers.first(where: { $0.id == activeProvider })?.available != true {
            activeProvider = providers.first(where: \.available)?.id ?? ""
        }
        persistWorkspace()
    }
    private func restoreSelection() {
        guard selectedSessionID == nil,
              let sessionID = lastSessions[activeProvider],
              sessions.contains(where: { $0.provider == activeProvider && $0.sessionId == sessionID }) else { return }
        selectedSessionID = sessionID
        sendSelection(sessionID)
    }
    private func persistWorkspace() {
        defaults.set(activeProvider, forKey: activeProviderKey)
        defaults.set(lastSessions, forKey: lastSessionsKey)
    }
    private func sendSelection(_ sessionID: String) {
        sendControl("select_session", ["sessionId": sessionID])
        sendControl("load_history", ["sessionId": sessionID, "limit": 500])
    }
    private func sendControl(_ action: String, _ fields: [String: Any] = [:]) { Task { try? await socket.send(.control(action: action, fields: fields)) } }
}
