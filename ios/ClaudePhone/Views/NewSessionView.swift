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
                Picker("权限", selection: $permission) {
                    ForEach(store.activeProviderInfo?.permissions ?? [], id: \.id) { option in
                        Text(option.dangerous ? "\(option.label) ⚠" : option.label).tag(option.id)
                    }
                }
                Button("创建会话") { store.create(project: store.projects.first { $0.path == projectPath }, permission: permission); dismiss() }.buttonStyle(.borderedProminent)
                    .disabled(permission.isEmpty)
            }
            .navigationTitle("新建会话")
            .toolbar { Button("取消") { dismiss() } }
            .onAppear { resetPermission() }
            .onChange(of: store.activeProvider) { _, _ in resetPermission() }
        }
    }

    private func resetPermission() {
        let options = store.activeProviderInfo?.permissions ?? []
        if !options.contains(where: { $0.id == permission }) {
            permission = options.first(where: { $0.id == "default" })?.id ?? options.first?.id ?? ""
        }
    }
}
