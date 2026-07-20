import SwiftUI

struct SessionListView: View {
    @Bindable var app: AppStore
    @State private var showingNew = false
    @State private var showingSettings = false
    var body: some View {
        NavigationStack {
            List(app.sessions.sessions) { session in
                NavigationLink { ChatView(app: app, session: session) } label: {
                    VStack(alignment: .leading) { Text(session.name); Text(session.status).font(.caption).foregroundStyle(.secondary) }
                }.simultaneousGesture(TapGesture().onEnded { app.sessions.select(session) })
            }
            .overlay { if app.sessions.sessions.isEmpty { ContentUnavailableView("暂无会话", systemImage: "bubble.left.and.bubble.right", description: Text("新建会话后即可从 iPhone 使用 Claude")) } }
            .navigationTitle("CodeAfar")
            .toolbar { ToolbarItemGroup(placement: .topBarTrailing) { Button { showingNew = true } label: { Image(systemName: "plus") }; Button { showingSettings = true } label: { Image(systemName: "gear") } } }
            .sheet(isPresented: $showingNew) { NewSessionView(store: app.sessions) }
            .sheet(isPresented: $showingSettings) { SettingsView(app: app) }
        }
    }
}
