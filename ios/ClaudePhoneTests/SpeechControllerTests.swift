import XCTest
@testable import ClaudePhone

final class SpeechControllerTests: XCTestCase {
    func testControllerStartsIdle() async { let controller = await SpeechController(); XCTAssertEqual(await controller.state, .idle) }
}
