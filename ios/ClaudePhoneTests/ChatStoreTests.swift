import XCTest
@testable import ClaudePhone

@MainActor final class ChatStoreTests: XCTestCase {
    func testHistoryAndQueueState() async {
        let socket = WebSocketClient(); let store = ChatStore(socket: socket)
        store.handle(.history(sessionID: "s", messages: [HistoryItem(type: "text", content: "hello", tool: nil, input: nil)]))
        store.handle(.queued(id: "q1", position: 1))
        XCTAssertEqual(store.messages.count, 2)
        store.handle(.dequeued(id: "q1"))
        XCTAssertEqual(store.messages.count, 1)
    }

    func testToolActivityIsIgnoredLiveAndInHistory() async {
        let socket = WebSocketClient(); let store = ChatStore(socket: socket)
        store.handle(.toolUse(tool: "Bash", input: #"{"command":"pwd"}"#))
        XCTAssertTrue(store.messages.isEmpty)
        store.handle(.history(sessionID: "s", messages: [
            HistoryItem(type: "text", content: "hello", tool: nil, input: nil),
            HistoryItem(type: "tool_use", content: nil, tool: "Read", input: "{}"),
            HistoryItem(type: "token", content: "done", tool: nil, input: nil),
        ]))
        try? await Task.sleep(for: .milliseconds(30))
        XCTAssertEqual(store.messages.map(\.text), ["hello", "done"])
    }
}
