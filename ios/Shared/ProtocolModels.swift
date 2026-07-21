import Foundation

struct SessionInfo: Codable, Identifiable, Hashable {
    let sessionId: String
    let name: String
    let status: String
    let owner: String
    let subscribers: [String]
    let createdAt: Int64
    let cwd: String
    let provider: String
    let model: String?
    let permissionMode: String
    var id: String { sessionId }
}

struct ProviderPermission: Codable, Identifiable, Hashable {
    let id: String
    let label: String
    let description: String
    let dangerous: Bool
    let mutable: Bool

    init(id: String, label: String, description: String = "", dangerous: Bool = false, mutable: Bool = true) {
        self.id = id; self.label = label; self.description = description; self.dangerous = dangerous; self.mutable = mutable
    }

    private enum CodingKeys: String, CodingKey { case id, label, description, dangerous, mutable }
    init(from decoder: Decoder) throws {
        let box = try decoder.container(keyedBy: CodingKeys.self)
        id = try box.decode(String.self, forKey: .id)
        label = try box.decode(String.self, forKey: .label)
        description = try box.decodeIfPresent(String.self, forKey: .description) ?? ""
        dangerous = try box.decodeIfPresent(Bool.self, forKey: .dangerous) ?? false
        mutable = try box.decodeIfPresent(Bool.self, forKey: .mutable) ?? true
    }
}

struct ProviderInfo: Codable, Identifiable, Hashable {
    let id: String
    let name: String
    let available: Bool
    let unavailableReason: String?
    let permissions: [ProviderPermission]

    init(id: String, name: String, available: Bool, unavailableReason: String?, permissions: [ProviderPermission]) {
        self.id = id; self.name = name; self.available = available; self.unavailableReason = unavailableReason; self.permissions = permissions
    }
}

struct ProjectInfo: Codable, Identifiable, Hashable {
    let name: String
    let path: String
    let permission: String?
    var id: String { path }
}

struct PromptTemplate: Codable, Identifiable, Hashable {
    let label: String
    let prompt: String
    var id: String { label + prompt }
}

struct HistoryItem: Codable, Identifiable, Hashable {
    let type: String
    let content: String?
    let tool: String?
    let input: String?
    var id = UUID()
    enum CodingKeys: String, CodingKey { case type, content, tool, input }
}

enum ServerMessage: Equatable {
    case hello(agentVersion: String, claudeVersion: String, protocolVersion: String)
    case sessionList([SessionInfo])
    case providerList([ProviderInfo])
    case projectList([ProjectInfo])
    case templateList([PromptTemplate])
    case sessionCreated(id: String, name: String, cwd: String, provider: String, model: String?, permissionMode: String)
    case sessionStopped(id: String)
    case history(sessionID: String, messages: [HistoryItem])
    case thinking
    case token(String)
    case toolUse(tool: String, input: String)
    case queued(id: String, position: Int)
    case dequeued(id: String)
    case health(sessionID: String, state: String, idleSeconds: Int64)
    case done
    case pong
    case error(code: String, message: String)
    case unknown(type: String)
}

extension ServerMessage: Decodable {
    private enum Keys: String, CodingKey {
        case type, agentVersion, claudeVersion, protocolVersion, sessions, projects, providers, templates
        case sessionId, name, cwd, provider, model, permissionMode, messages, content, tool, input, msgId, position, state, idleSeconds, code, message
    }

    init(from decoder: Decoder) throws {
        let box = try decoder.container(keyedBy: Keys.self)
        let type = try box.decode(String.self, forKey: .type)
        switch type {
        case "hello": self = .hello(agentVersion: try box.decode(String.self, forKey: .agentVersion), claudeVersion: try box.decode(String.self, forKey: .claudeVersion), protocolVersion: try box.decode(String.self, forKey: .protocolVersion))
        case "session_list": self = .sessionList(try box.decodeIfPresent([SessionInfo].self, forKey: .sessions) ?? [])
        case "provider_list": self = .providerList(try box.decodeIfPresent([ProviderInfo].self, forKey: .providers) ?? [])
        case "project_list": self = .projectList(try box.decodeIfPresent([ProjectInfo].self, forKey: .projects) ?? [])
        case "template_list": self = .templateList(try box.decodeIfPresent([PromptTemplate].self, forKey: .templates) ?? [])
        case "session_created": self = .sessionCreated(
            id: try box.decode(String.self, forKey: .sessionId),
            name: try box.decode(String.self, forKey: .name),
            cwd: try box.decode(String.self, forKey: .cwd),
            provider: try box.decode(String.self, forKey: .provider),
            model: try box.decodeIfPresent(String.self, forKey: .model),
            permissionMode: try box.decode(String.self, forKey: .permissionMode)
        )
        case "session_stopped": self = .sessionStopped(id: try box.decode(String.self, forKey: .sessionId))
        case "history": self = .history(sessionID: try box.decode(String.self, forKey: .sessionId), messages: try box.decodeIfPresent([HistoryItem].self, forKey: .messages) ?? [])
        case "thinking": self = .thinking
        case "token": self = .token(try box.decodeIfPresent(String.self, forKey: .content) ?? "")
        case "tool_use": self = .toolUse(tool: try box.decodeIfPresent(String.self, forKey: .tool) ?? "Tool", input: try box.decodeIfPresent(String.self, forKey: .input) ?? "")
        case "queued": self = .queued(id: try box.decode(String.self, forKey: .msgId), position: try box.decode(Int.self, forKey: .position))
        case "dequeued": self = .dequeued(id: try box.decode(String.self, forKey: .msgId))
        case "health": self = .health(sessionID: try box.decode(String.self, forKey: .sessionId), state: try box.decode(String.self, forKey: .state), idleSeconds: try box.decode(Int64.self, forKey: .idleSeconds))
        case "done": self = .done
        case "pong": self = .pong
        case "error": self = .error(code: try box.decode(String.self, forKey: .code), message: try box.decode(String.self, forKey: .message))
        default: self = .unknown(type: type)
        }
    }
}

enum ClientMessage {
    case auth(deviceToken: String, deviceName: String)
    case control(action: String, fields: [String: Any] = [:])
    case text(String)

    func data() throws -> Data {
        var value: [String: Any]
        switch self {
        case .auth(let token, let name): value = ["type": "auth", "deviceToken": token, "deviceName": name]
        case .control(let action, let fields): value = ["type": "control", "action": action].merging(fields) { _, new in new }
        case .text(let content): value = ["type": "text", "content": content]
        }
        return try JSONSerialization.data(withJSONObject: value, options: [.sortedKeys])
    }
}
