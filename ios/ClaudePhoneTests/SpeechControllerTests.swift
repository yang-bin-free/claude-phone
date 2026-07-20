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
    private var onText: ((String) -> Void)?

    init(available: Bool = true) { self.available = available }

    var isAvailable: Bool { get async { available } }

    func start(onText: @escaping (String) -> Void, onFinish: @escaping (Error?) -> Void) async throws {
        startCount += 1
        self.onText = onText
    }

    func stop() async {
        stopCount += 1
    }

    func emit(_ text: String) { onText?(text) }
}
