import XCTest
@testable import ClaudePhone

@MainActor
final class SpeechControllerTests: XCTestCase {
    func testControllerStartsIdle() {
        let controller = SpeechController(engineFactory: { FakeOnDeviceSpeechEngine() }, authorize: { true })
        XCTAssertEqual(controller.state, .idle)
    }

    func testPartialTextIsReturnedWithoutSending() async {
        let fake = FakeOnDeviceSpeechEngine()
        let controller = SpeechController(engineFactory: { fake }, authorize: { true })
        var values: [String] = []
        controller.onText = { values.append($0) }

        await controller.start()
        fake.emit("你好")

        XCTAssertEqual(values, ["你好"])
        XCTAssertEqual(controller.state, .listening)
        XCTAssertEqual(fake.startCount, 1)
    }

    func testUnavailableEngineDoesNotStart() async {
        let fake = FakeOnDeviceSpeechEngine(available: false)
        let controller = SpeechController(engineFactory: { fake }, authorize: { true })

        await controller.start()

        XCTAssertEqual(controller.state, .unavailable("当前设备或语言不支持离线语音输入"))
        XCTAssertEqual(fake.startCount, 0)
    }

    func testDeniedPermissionDoesNotConstructEngine() async {
        var factoryCalls = 0
        let controller = SpeechController(engineFactory: {
            factoryCalls += 1
            return FakeOnDeviceSpeechEngine()
        }, authorize: { false })

        await controller.start()

        XCTAssertEqual(controller.state, .denied)
        XCTAssertEqual(factoryCalls, 0)
    }

    func testStopCleansUpTheActiveEngine() async {
        let fake = FakeOnDeviceSpeechEngine()
        let controller = SpeechController(engineFactory: { fake }, authorize: { true })
        await controller.start()

        await controller.stop()

        XCTAssertEqual(fake.stopCount, 1)
        XCTAssertEqual(controller.state, .idle)
    }

    func testFailedStartCleansUpPartiallyStartedEngine() async {
        let fake = FakeOnDeviceSpeechEngine(startError: TestError.failed)
        let controller = SpeechController(engineFactory: { fake }, authorize: { true })

        await controller.start()

        XCTAssertEqual(fake.startCount, 1)
        XCTAssertEqual(fake.stopCount, 1)
        if case .failed = controller.state {} else { XCTFail("expected failed state") }
    }

    func testConcurrentStartsCreateOnlyOneEngine() async {
        let fake = FakeOnDeviceSpeechEngine()
        let gate = AuthorizationGate()
        let controller = SpeechController(engineFactory: { fake }, authorize: { await gate.wait() })

        async let first: Void = controller.start()
        async let second: Void = controller.start()
        await gate.open()
        _ = await (first, second)

        XCTAssertEqual(fake.startCount, 1)
    }

    func testStaleFinishCannotStopNewRecording() async {
        let first = FakeOnDeviceSpeechEngine()
        let second = FakeOnDeviceSpeechEngine()
        var engines: [FakeOnDeviceSpeechEngine] = [first, second]
        let controller = SpeechController(engineFactory: { engines.removeFirst() }, authorize: { true })
        await controller.start()
        await controller.stop()
        await controller.start()

        first.emitFinish()
        await Task.yield()

        XCTAssertEqual(second.stopCount, 0)
        XCTAssertEqual(controller.state, .listening)
    }

    func testStaleTextCannotOverwriteNewRecording() async {
        let first = FakeOnDeviceSpeechEngine()
        let second = FakeOnDeviceSpeechEngine()
        var engines: [FakeOnDeviceSpeechEngine] = [first, second]
        let controller = SpeechController(engineFactory: { engines.removeFirst() }, authorize: { true })
        var values: [String] = []
        controller.onText = { values.append($0) }
        await controller.start()
        await controller.stop()
        await controller.start()

        first.emit("过期文本")
        second.emit("当前文本")

        XCTAssertEqual(values, ["当前文本"])
    }

    func testFactoryRoutesIOS26ToSpeechAnalyzer() {
        XCTAssertEqual(SpeechEngineKind.forMajorVersion(25), .legacyOnDevice)
        XCTAssertEqual(SpeechEngineKind.forMajorVersion(26), .speechAnalyzer)
    }

    func testSpeechTextAppendsToExistingDraft() {
        XCTAssertEqual(mergeSpeechDraft(base: "请修复这个问题", transcript: "并运行测试"), "请修复这个问题 并运行测试")
        XCTAssertEqual(mergeSpeechDraft(base: "", transcript: "你好"), "你好")
    }
}

@MainActor
private final class FakeOnDeviceSpeechEngine: OnDeviceSpeechEngine {
    let available: Bool
    var startCount = 0
    var stopCount = 0
    let startError: Error?
    private var onText: ((String) -> Void)?
    private var onFinish: ((Error?) -> Void)?

    init(available: Bool = true, startError: Error? = nil) {
        self.available = available
        self.startError = startError
    }

    var isAvailable: Bool { get async { available } }

    func start(onText: @escaping (String) -> Void, onFinish: @escaping (Error?) -> Void) async throws {
        startCount += 1
        if let startError { throw startError }
        self.onText = onText
        self.onFinish = onFinish
    }

    func stop() async {
        stopCount += 1
    }

    func emit(_ text: String) { onText?(text) }
    func emitFinish(_ error: Error? = nil) { onFinish?(error) }
}

private enum TestError: Error { case failed }

private actor AuthorizationGate {
    private var continuation: CheckedContinuation<Bool, Never>?
	private var opened = false
	func wait() async -> Bool {
		if opened { return true }
		return await withCheckedContinuation { continuation = $0 }
	}
	func open() { opened = true; continuation?.resume(returning: true); continuation = nil }
}
