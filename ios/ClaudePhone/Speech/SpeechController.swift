import AVFoundation
import Observation
import Speech

@MainActor
protocol OnDeviceSpeechEngine: AnyObject {
    var isAvailable: Bool { get async }
    func start(
        onText: @escaping (String) -> Void,
        onFinish: @escaping (Error?) -> Void
    ) async throws
    func stop() async
}

enum SpeechEngineKind: Equatable {
    case legacyOnDevice
    case speechAnalyzer

    static func forMajorVersion(_ majorVersion: Int) -> Self {
        majorVersion >= 26 ? .speechAnalyzer : .legacyOnDevice
    }
}

@MainActor @Observable
final class SpeechController {
    enum State: Equatable {
        case idle
        case requestingPermission
        case preparing
        case listening
        case unavailable(String)
        case denied
        case failed(String)
    }

    private(set) var state: State = .idle
    private let engineFactory: @MainActor () -> any OnDeviceSpeechEngine
    private let authorize: () async -> Bool
    private var activeEngine: (any OnDeviceSpeechEngine)?
    private var starting = false
    private var generation = 0
    var onText: ((String) -> Void)?

    init(
        engineFactory: @escaping @MainActor () -> any OnDeviceSpeechEngine = SpeechController.defaultEngine,
        authorize: @escaping () async -> Bool = SpeechController.requestAuthorization
    ) {
        self.engineFactory = engineFactory
        self.authorize = authorize
    }

    func start() async {
        if activeEngine != nil {
            await stop()
            return
        }
        guard !starting else { return }
        starting = true
        generation += 1
        let startGeneration = generation

        state = .requestingPermission
        guard await authorize() else {
            guard generation == startGeneration else { return }
            starting = false
            state = .denied
            return
        }
        guard generation == startGeneration, starting else { return }

        state = .preparing
        let engine = engineFactory()
        activeEngine = engine
        let engineID = ObjectIdentifier(engine)
        guard await engine.isAvailable else {
            guard generation == startGeneration else { await engine.stop(); return }
            activeEngine = nil
            starting = false
            await engine.stop()
            state = .unavailable("当前设备或语言不支持离线语音输入")
            return
        }

        do {
            try await engine.start(
                onText: { [weak self] text in
                    self?.acceptText(text, engineID: engineID, generation: startGeneration)
                },
                onFinish: { [weak self] error in
                    Task { @MainActor in
                        await self?.finish(engineID: engineID, generation: startGeneration, error: error)
                    }
                }
            )
            guard generation == startGeneration, starting else { await engine.stop(); return }
            starting = false
            state = .listening
        } catch {
            await engine.stop()
            guard generation == startGeneration else { return }
            activeEngine = nil
            starting = false
            state = .failed(error.localizedDescription)
        }
    }

    func stop() async {
        generation += 1
        starting = false
        let engine = activeEngine
        activeEngine = nil
        await engine?.stop()
        state = .idle
    }

    private func finish(engineID: ObjectIdentifier, generation callbackGeneration: Int, error: Error?) async {
        guard generation == callbackGeneration, let activeEngine, ObjectIdentifier(activeEngine) == engineID else { return }
        let engine = activeEngine
        self.activeEngine = nil
        starting = false
        await engine.stop()
        if let error {
            state = .failed(error.localizedDescription)
        } else {
            state = .idle
        }
    }

    private func acceptText(_ text: String, engineID: ObjectIdentifier, generation callbackGeneration: Int) {
        guard generation == callbackGeneration, let activeEngine, ObjectIdentifier(activeEngine) == engineID else { return }
        onText?(text)
    }

    private static func requestAuthorization() async -> Bool {
        let speech = await withCheckedContinuation { continuation in
            SFSpeechRecognizer.requestAuthorization { continuation.resume(returning: $0) }
        }
        guard speech == .authorized else { return false }
        return await withCheckedContinuation { continuation in
            AVAudioApplication.requestRecordPermission { continuation.resume(returning: $0) }
        }
    }

    private static func defaultEngine() -> any OnDeviceSpeechEngine {
        if #available(iOS 26.0, *) {
            return SpeechAnalyzerOnDeviceEngine()
        }
        return LegacyOnDeviceSpeechEngine()
    }
}

private enum OnDeviceSpeechError: LocalizedError {
    case unavailable
    case missingAssets
    case unsupportedAudioFormat

    var errorDescription: String? {
        switch self {
        case .unavailable:
            return "当前设备或语言不支持离线语音输入"
        case .missingAssets:
            return "无法下载 Apple 本地语音资源"
        case .unsupportedAudioFormat:
            return "当前麦克风格式不支持本地语音输入"
        }
    }
}

@MainActor
private final class LegacyOnDeviceSpeechEngine: OnDeviceSpeechEngine {
    private let recognizer = SFSpeechRecognizer(locale: .current)
    private let audioEngine = AVAudioEngine()
    private var request: SFSpeechAudioBufferRecognitionRequest?
    private var recognitionTask: SFSpeechRecognitionTask?
    private var hasTap = false

    var isAvailable: Bool {
        get async {
            recognizer?.isAvailable == true && recognizer?.supportsOnDeviceRecognition == true
        }
    }

    func start(
        onText: @escaping (String) -> Void,
        onFinish: @escaping (Error?) -> Void
    ) async throws {
        guard await isAvailable else { throw OnDeviceSpeechError.unavailable }

        let audioSession = AVAudioSession.sharedInstance()
        try audioSession.setCategory(.record, mode: .measurement, options: .duckOthers)
        try audioSession.setActive(true, options: .notifyOthersOnDeactivation)

        let request = SFSpeechAudioBufferRecognitionRequest()
        request.shouldReportPartialResults = true
        request.requiresOnDeviceRecognition = true
        self.request = request

        let inputNode = audioEngine.inputNode
        let format = inputNode.outputFormat(forBus: 0)
        inputNode.installTap(onBus: 0, bufferSize: 1024, format: format) { buffer, _ in
            request.append(buffer)
        }
        hasTap = true
        audioEngine.prepare()
        try audioEngine.start()

        recognitionTask = recognizer?.recognitionTask(with: request) { result, error in
            Task { @MainActor in
                if let text = result?.bestTranscription.formattedString {
                    onText(text)
                }
                if let error {
                    onFinish(error)
                } else if result?.isFinal == true {
                    onFinish(nil)
                }
            }
        }
    }

    func stop() async {
        audioEngine.stop()
        if hasTap {
            audioEngine.inputNode.removeTap(onBus: 0)
            hasTap = false
        }
        request?.endAudio()
        recognitionTask?.cancel()
        request = nil
        recognitionTask = nil
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
    }
}

@available(iOS 26.0, *)
@MainActor
private final class SpeechAnalyzerOnDeviceEngine: OnDeviceSpeechEngine {
    private let audioEngine = AVAudioEngine()
    private var analyzer: SpeechAnalyzer?
    private var inputContinuation: AsyncStream<AnalyzerInput>.Continuation?
    private var analysisTask: Task<Void, Never>?
    private var resultsTask: Task<Void, Never>?
    private var hasTap = false

    var isAvailable: Bool {
        get async {
            guard SpeechTranscriber.isAvailable else { return false }
            return await SpeechTranscriber.supportedLocale(equivalentTo: .current) != nil
        }
    }

    func start(
        onText: @escaping (String) -> Void,
        onFinish: @escaping (Error?) -> Void
    ) async throws {
        guard SpeechTranscriber.isAvailable,
              let locale = await SpeechTranscriber.supportedLocale(equivalentTo: .current)
        else { throw OnDeviceSpeechError.unavailable }

        let transcriber = SpeechTranscriber(locale: locale, preset: .progressiveTranscription)
        let modules: [any SpeechModule] = [transcriber]
        let assetStatus = await AssetInventory.status(forModules: modules)
        if assetStatus == .unsupported { throw OnDeviceSpeechError.unavailable }
        if assetStatus != .installed {
            guard let request = try await AssetInventory.assetInstallationRequest(supporting: modules)
            else { throw OnDeviceSpeechError.missingAssets }
            try await request.downloadAndInstall()
        }

        let audioSession = AVAudioSession.sharedInstance()
        try audioSession.setCategory(.record, mode: .measurement, options: .duckOthers)
        try audioSession.setActive(true, options: .notifyOthersOnDeactivation)

        let inputNode = audioEngine.inputNode
        let naturalFormat = inputNode.outputFormat(forBus: 0)
        guard let targetFormat = await SpeechAnalyzer.bestAvailableAudioFormat(
            compatibleWith: modules,
            considering: naturalFormat
        ) else { throw OnDeviceSpeechError.unsupportedAudioFormat }

        let stream = AsyncStream<AnalyzerInput> { continuation in
            inputContinuation = continuation
        }
        let analyzer = SpeechAnalyzer(modules: modules)
        self.analyzer = analyzer
        try await analyzer.prepareToAnalyze(in: targetFormat)

        let converter = AVAudioConverter(from: naturalFormat, to: targetFormat)
        inputNode.installTap(onBus: 0, bufferSize: 1024, format: naturalFormat) { [weak self] buffer, _ in
            guard let self else { return }
            if naturalFormat == targetFormat {
                self.inputContinuation?.yield(AnalyzerInput(buffer: buffer))
                return
            }
            guard let converter,
                  let converted = AVAudioPCMBuffer(
                      pcmFormat: targetFormat,
                      frameCapacity: AVAudioFrameCount(
                          Double(buffer.frameLength) * targetFormat.sampleRate / naturalFormat.sampleRate + 1
                      )
                  )
            else { return }
            var supplied = false
            var conversionError: NSError?
            let status = converter.convert(to: converted, error: &conversionError) { _, inputStatus in
                if supplied {
                    inputStatus.pointee = .noDataNow
                    return nil
                }
                supplied = true
                inputStatus.pointee = .haveData
                return buffer
            }
            if status == .haveData {
                self.inputContinuation?.yield(AnalyzerInput(buffer: converted))
            }
        }
        hasTap = true

        resultsTask = Task { @MainActor in
            do {
                for try await result in transcriber.results {
                    onText(String(result.text.characters))
                }
            } catch is CancellationError {
                return
            } catch {
                onFinish(error)
            }
        }
        analysisTask = Task { @MainActor in
            do {
                try await analyzer.start(inputSequence: stream)
            } catch is CancellationError {
                return
            } catch {
                onFinish(error)
            }
        }

        audioEngine.prepare()
        try audioEngine.start()
    }

    func stop() async {
        audioEngine.stop()
        if hasTap {
            audioEngine.inputNode.removeTap(onBus: 0)
            hasTap = false
        }
        inputContinuation?.finish()
        inputContinuation = nil
        if let analyzer {
            try? await analyzer.finalizeAndFinishThroughEndOfInput()
        }
        analysisTask?.cancel()
        resultsTask?.cancel()
        analysisTask = nil
        resultsTask = nil
        self.analyzer = nil
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
    }
}
