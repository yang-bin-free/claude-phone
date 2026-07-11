import Foundation

enum AppGroup {
    static let identifier = Bundle.main.object(forInfoDictionaryKey: "AppGroupIdentifier") as? String
        ?? "group.com.example.ClaudePhone"

    static var defaults: UserDefaults {
        guard let defaults = UserDefaults(suiteName: identifier) else {
            preconditionFailure("Unable to open App Group \(identifier)")
        }
        return defaults
    }
}

enum SharedKey {
    static let authKey = "transientAuthKey"
    static let controlURL = "controlURL"
    static let stateDirectory = "tailscaleStateDirectory"
    static let tunnelState = "tunnelState"
    static let tunnelError = "tunnelError"
}
