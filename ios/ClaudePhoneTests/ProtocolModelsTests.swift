import XCTest
@testable import ClaudePhone

final class ProtocolModelsTests: XCTestCase {
    func testDecodesTokenAndUnknownMessage() throws {
        XCTAssertEqual(try JSONDecoder().decode(ServerMessage.self, from: Data(#"{"type":"token","content":"hi","future":true}"#.utf8)), .token("hi"))
        XCTAssertEqual(try JSONDecoder().decode(ServerMessage.self, from: Data(#"{"type":"future"}"#.utf8)), .unknown(type: "future"))
    }
    func testRetryPolicyCapsAtFifteenSeconds() { XCTAssertEqual(RetryPolicy.delay(attempt: 99), .seconds(15)) }
}
