import AVFoundation
import Observation
import Speech

@MainActor @Observable
final class SpeechController {
    enum State: Equatable { case idle, listening, denied, failed(String) }
    private(set) var state: State = .idle
    private let recognizer = SFSpeechRecognizer()
    private let engine = AVAudioEngine()
    private var request: SFSpeechAudioBufferRecognitionRequest?
    private var task: SFSpeechRecognitionTask?
    var onText: ((String) -> Void)?

    func start() async {
        let speech = await withCheckedContinuation { continuation in SFSpeechRecognizer.requestAuthorization { continuation.resume(returning: $0) } }
        let microphone = await withCheckedContinuation { continuation in AVAudioApplication.requestRecordPermission { continuation.resume(returning: $0) } }
        guard speech == .authorized, microphone else { state = .denied; return }
        do { try beginRecognition() } catch { state = .failed(error.localizedDescription) }
    }
    func stop() { engine.stop(); engine.inputNode.removeTap(onBus: 0); request?.endAudio(); task?.cancel(); request = nil; task = nil; state = .idle }
    private func beginRecognition() throws {
        stop()
        let request = SFSpeechAudioBufferRecognitionRequest(); request.shouldReportPartialResults = true; self.request = request
        let node = engine.inputNode; let format = node.outputFormat(forBus: 0)
        node.installTap(onBus: 0, bufferSize: 1024, format: format) { buffer, _ in request.append(buffer) }
        engine.prepare(); try engine.start(); state = .listening
        task = recognizer?.recognitionTask(with: request) { [weak self] result, error in Task { @MainActor in
            if let text = result?.bestTranscription.formattedString { self?.onText?(text) }
            if error != nil || result?.isFinal == true { self?.stop() }
        } }
    }
}
