import SwiftUI

struct ToolUseCard: View {
    let tool: String
    let input: String
    var body: some View { DisclosureGroup("🔧 \(tool)") { Text(input).font(.system(.caption, design: .monospaced)).textSelection(.enabled) }.padding().background(.purple.opacity(0.1), in: RoundedRectangle(cornerRadius: 12)) }
}
