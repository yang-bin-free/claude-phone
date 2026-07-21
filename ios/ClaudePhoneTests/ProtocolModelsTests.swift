import XCTest
@testable import ClaudePhone

final class ProtocolModelsTests: XCTestCase {
    func testDecodesTokenAndUnknownMessage() throws {
        XCTAssertEqual(try JSONDecoder().decode(ServerMessage.self, from: Data(#"{"type":"token","content":"hi","future":true}"#.utf8)), .token("hi"))
        XCTAssertEqual(try JSONDecoder().decode(ServerMessage.self, from: Data(#"{"type":"future"}"#.utf8)), .unknown(type: "future"))
    }
    func testDecodesProviderAndProviderBearingSession() throws {
        let provider = try JSONDecoder().decode(ServerMessage.self, from: Data(#"{"type":"provider_list","providers":[{"id":"codex","name":"Codex","available":true,"permissions":[]}]}"#.utf8))
        XCTAssertEqual(provider, .providerList([
            ProviderInfo(id: "codex", name: "Codex", available: true, unavailableReason: nil, permissions: [])
        ]))
        let session = try JSONDecoder().decode(ServerMessage.self, from: Data(#"{"type":"session_list","sessions":[{"sessionId":"x1","name":"Task","status":"active","owner":"Mac","subscribers":[],"createdAt":1,"cwd":"/tmp","provider":"codex","permissionMode":"workspaceWrite"}]}"#.utf8))
        guard case .sessionList(let sessions) = session else { return XCTFail("expected session_list") }
        XCTAssertEqual(sessions.first?.provider, "codex")
        XCTAssertEqual(sessions.first?.permissionMode, "workspaceWrite")
    }
    func testRetryPolicyCapsAtFifteenSeconds() { XCTAssertEqual(RetryPolicy.delay(attempt: 99), .seconds(15)) }
}
