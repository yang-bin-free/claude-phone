import SwiftUI

struct SessionListView: View {
    @Bindable var app: AppStore
    @State private var showingNew = false
    @State private var showingSettings = false
    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                HStack(spacing: 8) {
                    HStack(spacing: 3) {
                        ForEach(app.sessions.providers) { provider in
                            Button {
                                app.sessions.switchProvider(provider.id)
                            } label: {
                                Text(provider.id == "claude" ? "Claude" : provider.name)
                                    .font(.subheadline.weight(.semibold))
                                    .frame(maxWidth: .infinity)
                                    .padding(.vertical, 8)
                                    .background(app.sessions.activeProvider == provider.id ? Color.accentColor.opacity(0.22) : Color.clear)
                                    .clipShape(.rect(cornerRadius: 8))
                            }
                            .buttonStyle(.plain)
                            .disabled(!provider.available)
                            .accessibilityHint(provider.unavailableReason ?? "")
                        }
                    }
                    .padding(3)
                    .background(.secondary.opacity(0.12))
                    .clipShape(.rect(cornerRadius: 11))
                    Button { showingNew = true } label: { Image(systemName: "plus").frame(width: 36, height: 36) }
                        .buttonStyle(.borderedProminent)
                        .accessibilityLabel("在当前引擎中新建会话")
                }
                .padding(.horizontal)
                .padding(.vertical, 8)
                List(app.sessions.visibleSessions) { session in
                    NavigationLink { ChatView(app: app, session: session) } label: {
                        VStack(alignment: .leading) { Text(session.name); Text(session.status).font(.caption).foregroundStyle(.secondary) }
                    }.simultaneousGesture(TapGesture().onEnded { app.sessions.select(session) })
                }
                .overlay {
                    if app.sessions.visibleSessions.isEmpty {
                        ContentUnavailableView("暂无会话", systemImage: "bubble.left.and.bubble.right", description: Text("在当前引擎中新建一个会话"))
                    }
                }
            }
            .navigationTitle("CodeAfar")
            .toolbar { Button { showingSettings = true } label: { Image(systemName: "gear") } }
            .sheet(isPresented: $showingNew) { NewSessionView(store: app.sessions) }
            .sheet(isPresented: $showingSettings) { SettingsView(app: app) }
        }
    }
}
