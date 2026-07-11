import SwiftUI

struct NewSessionView: View {
    @Bindable var store: SessionStore
    @State private var projectPath = ""
    @State private var permission = "default"
    @Environment(\.dismiss) private var dismiss
    var body: some View {
        NavigationStack {
            Form {
                Picker("项目", selection: $projectPath) { Text("默认目录").tag(""); ForEach(store.projects) { Text($0.name).tag($0.path) } }
                Picker("权限", selection: $permission) { Text("严格").tag("default"); Text("审阅").tag("acceptEdits"); Text("信任").tag("bypassPermissions") }
                Button("创建会话") { store.create(project: store.projects.first { $0.path == projectPath }, permission: permission); dismiss() }.buttonStyle(.borderedProminent)
            }.navigationTitle("新建会话").toolbar { Button("取消") { dismiss() } }
        }
    }
}
