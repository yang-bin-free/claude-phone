import SwiftUI

struct ChatView: View {
    @Bindable var app: AppStore
    let session: SessionInfo
    var body: some View {
        VStack(spacing: 0) {
            if app.chat.healthState != "healthy" { Text(app.chat.healthState == "stalled" ? "会话可能卡住" : "会话无响应").font(.caption).frame(maxWidth: .infinity).padding(6).background(.orange.opacity(0.2)) }
            ScrollViewReader { proxy in
                ScrollView { LazyVStack { ForEach(app.chat.messages) { MessageRow(message: $0).id($0.id) } }.padding() }
                    .onChange(of: app.chat.messages.count) { _, _ in if let id = app.chat.messages.last?.id { proxy.scrollTo(id, anchor: .bottom) } }
            }
            ScrollView(.horizontal, showsIndicators: false) { HStack { ForEach(app.sessions.templates) { template in Button(template.label) { app.chat.composer = template.prompt }.buttonStyle(.bordered) } }.padding(.horizontal) }
            if let speechStatus { Text(speechStatus).font(.caption).foregroundStyle(.secondary).padding(.horizontal) }
            HStack(alignment: .bottom) { TextField("输入消息", text: Binding(get: { app.chat.composer }, set: { app.chat.composer = $0 }), axis: .vertical).textFieldStyle(.roundedBorder); Button { Task { await app.toggleSpeech() } } label: { Image(systemName: app.speech.state == .listening ? "stop.circle.fill" : "mic.fill") }; Button { app.chat.send() } label: { Image(systemName: "arrow.up.circle.fill").font(.title2) } }.padding()
        }
        .navigationTitle(session.name)
        .toolbar { Button(role: .destructive) { app.sessions.stop(session.sessionId) } label: { Image(systemName: "stop.circle") } }
        .onDisappear { Task { await app.speech.stop() } }
    }

    private var speechStatus: String? {
        switch app.speech.state {
        case .idle: nil
        case .requestingPermission: "正在请求麦克风权限…"
        case .preparing: "正在准备本地语音模型…"
        case .listening: "正在本地识别，点击停止"
        case .unavailable(let message), .failed(let message): message
        case .denied: "需要语音识别和麦克风权限"
        }
    }
}
