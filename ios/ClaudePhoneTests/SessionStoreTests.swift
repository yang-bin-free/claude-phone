import XCTest
@testable import ClaudePhone

@MainActor final class SessionStoreTests: XCTestCase {
    func testProviderSwitchFiltersAndRestoresSessions() {
        let suite = "SessionStoreTests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let store = SessionStore(socket: WebSocketClient(), defaults: defaults)
        store.handle(.providerList([
            ProviderInfo(id: "claude", name: "Claude Code", available: true, unavailableReason: nil, permissions: []),
            ProviderInfo(id: "codex", name: "Codex", available: true, unavailableReason: nil, permissions: []),
        ]))
        store.handle(.sessionList([
            SessionInfo(sessionId: "c1", name: "Claude task", status: "active", owner: "Mac", subscribers: [], createdAt: 1, cwd: "/c", provider: "claude", model: nil, permissionMode: "default"),
            SessionInfo(sessionId: "x1", name: "Codex task", status: "active", owner: "Mac", subscribers: [], createdAt: 2, cwd: "/x", provider: "codex", model: nil, permissionMode: "workspaceWrite"),
        ]))
        store.select(store.sessions[0])
        store.switchProvider("codex")
        XCTAssertEqual(store.visibleSessions.map(\.sessionId), ["x1"])
        store.select(store.sessions[1])
        store.switchProvider("claude")
        XCTAssertEqual(store.selectedSessionID, "c1")
    }

    func testUnavailableProviderCannotBeSelected() {
        let suite = "SessionStoreTests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let store = SessionStore(socket: WebSocketClient(), defaults: defaults)
        store.handle(.providerList([
            ProviderInfo(id: "claude", name: "Claude Code", available: true, unavailableReason: nil, permissions: []),
            ProviderInfo(id: "codex", name: "Codex", available: false, unavailableReason: "missing", permissions: []),
        ]))
        store.switchProvider("codex")
        XCTAssertEqual(store.activeProvider, "claude")
    }
}
