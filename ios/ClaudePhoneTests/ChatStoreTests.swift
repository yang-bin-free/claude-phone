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
}
