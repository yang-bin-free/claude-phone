import Foundation

protocol SharedConfiguring {
    func stageAuthKey(_ value: String)
    func consumeAuthKey() -> String?
    func set(_ value: String?, for key: String)
    func string(for key: String) -> String?
    func clear()
}

final class SharedConfiguration: SharedConfiguring, @unchecked Sendable {
    private let defaults: UserDefaults
    private let lock = NSLock()

    init(defaults: UserDefaults = AppGroup.defaults) { self.defaults = defaults }
    func stageAuthKey(_ value: String) { set(value, for: SharedKey.authKey) }
    func consumeAuthKey() -> String? {
        lock.lock(); defer { lock.unlock() }
        let value = defaults.string(forKey: SharedKey.authKey)
        defaults.removeObject(forKey: SharedKey.authKey)
        return value
    }
    func set(_ value: String?, for key: String) { lock.lock(); defer { lock.unlock() }; defaults.set(value, forKey: key) }
    func string(for key: String) -> String? { lock.lock(); defer { lock.unlock() }; return defaults.string(forKey: key) }
    func clear() { lock.lock(); defer { lock.unlock() }; for key in [SharedKey.authKey, SharedKey.controlURL, SharedKey.stateDirectory, SharedKey.tunnelState, SharedKey.tunnelError] { defaults.removeObject(forKey: key) } }
}
