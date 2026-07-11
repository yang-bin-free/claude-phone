import SwiftUI

struct MessageRow: View {
    let message: ChatMessage
    var body: some View {
        HStack { if message.role == .user { Spacer(minLength: 40) }; Text(message.text).textSelection(.enabled).padding(12).background(background, in: RoundedRectangle(cornerRadius: 14)); if message.role != .user { Spacer(minLength: 40) } }
    }
    private var background: Color {
        switch message.role { case .user: .accentColor.opacity(0.85); case .error: .red.opacity(0.2); case .tool: .purple.opacity(0.15); case .status: .gray.opacity(0.15); case .assistant: .secondary.opacity(0.12) }
    }
}
