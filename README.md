# Claude Phone — 手机控制 Mac Claude Code

> 设计方案 v1 | 2026-07-07 | 阿彬 & 小罗

---

## 1. 目标

用手机远程控制 Mac 上的 Claude Code，进行开发协作。

**最终体验**：
- 手机：只安装 **1 个 APK**（Claude Phone）
- Mac：只运行 **1 个二进制**（claude-phone-agent）+ Claude Code CLI
- 不需要装 Tailscale App、不需要装 SSH 客户端、不需要配置端口映射、不需要记忆 IP 地址

**核心能力**：
1. 创建新会话
2. 切换已有会话
3. 停止会话
4. 流式查看 Claude 回复
5. 语音输入

---

## 2. 架构

```
┌─────────────────────────────────┐      ┌──────────────────────────┐
│  Android APK (Claude Phone)      │      │  Mac                      │
│                                  │      │                          │
│  ┌────────────────────────────┐  │      │  claude-phone-agent      │
│  │ Kotlin Shell (~50行)        │  │      │  (纯 Go 二进制, ~500行)   │
│  │  - WebView                  │  │      │                          │
│  │  - 麦克风权限                │  │      │  ┌────────────────────┐ │
│  │  - VpnService               │  │      │  │ tsnet.Server{}     │ │
│  └──────────┬─────────────────┘  │      │  │ 加入 Tailscale 网络  │ │
│             │ JS Bridge          │      │  │ Hostname: claude-mac│ │
│  ┌──────────▼─────────────────┐  │      │  └────────┬───────────┘ │
│  │ Go 核心 (gomobile → .aar)   │  │      │           │             │
│  │                            │  │      │  ┌────────▼───────────┐ │
│  │  tsnet.Server{}            │  │      │  │ WebSocket Server   │ │
│  │  WebSocket Client          │  │      │  │ 监听 :9876          │ │
│  │  协议处理 (共享 pkg/protocol)│  │      │  └────────┬───────────┘ │
│  └────────────────────────────┘  │      │           │             │
│                                  │      │  ┌────────▼───────────┐ │
│  ┌────────────────────────────┐  │      │  │ Session Manager    │ │
│  │ Chat UI (WebView)           │  │      │  │ map[sessionId]→    │ │
│  │  HTML/CSS/JS (AI 生成维护)   │  │      │  │   ClaudeSession    │ │
│  │  Web Speech API (语音输入)   │  │      │  │ 按需启动/唤醒/销毁  │ │
│  └────────────────────────────┘  │      │  └────────────────────┘ │
│                                  │      │                          │
└──────────────────────────────────┘      └──────────────────────────┘
         │                                        │
         │          WireGuard 加密直连              │
         └──────────────┬─────────────────────────┘
                        │
              Tailscale 协调服务器
              (只做地址交换 + 密钥分发, 不传数据)
```

### 为什么全 Go

- Tailscale 的 `tsnet` 包只需几行代码即可将 Go 进程加入 Tailscale 网络
- 手机端用 `gomobile` 将 Go 核心编译为 Android `.aar`，通过 JS Bridge 与 WebView 通信
- 协议和会话管理代码两端共享（`pkg/protocol`、`pkg/session`）
- 整个项目一个语言、一个工具链、零外部运行时依赖

### tsnet — 核心黑科技

```go
// Mac 端: 3 行代码加入 Tailscale 网络
s := &tsnet.Server{Hostname: "claude-mac", AuthKey: key}
ln, _ := s.Listen("tcp", ":9876")

// Android 端: 同样是 3 行
s := &tsnet.Server{Hostname: "claude-phone", AuthKey: key}
conn, _ := s.Dial("tcp", "claude-mac:9876") // 直接用主机名连接！
```

`tsnet` 自动处理 NAT 穿透、WireGuard 加密、密钥交换。拿到的是普通 `net.Conn`，读写 TCP 就行。

---

## 3. 项目结构

```
claude-phone/                       ← GitHub: github.com/yang-bin-free/claude-phone
├── README.md
├── LICENSE (MIT)
├── go.mod
│
├── cmd/
│   ├── mac-agent/main.go           ← Mac 端助手（单个二进制）
│   └── android-lib/main.go         ← Android Go 核心（gomobile → .aar）
│
├── pkg/
│   ├── protocol/messages.go        ← ★ 两端共享: JSON 消息定义
│   └── session/manager.go          ← ★ 两端共享: 会话管理
│
├── android/                        ← Android 壳
│   └── app/src/main/
│       ├── java/.../MainActivity.kt  ← ~50 行 Kotlin
│       └── AndroidManifest.xml
│
├── web/chat/                       ← 聊天 UI (AI 生成 + 维护)
│   ├── index.html
│   ├── chat.css
│   └── chat.js
│
└── scripts/build.sh                ← gomobile bind + gradle assemble
```

### 依赖与许可证

| 依赖 | 用途 | 许可证 |
|------|------|--------|
| `tailscale.com/tsnet` | Tailscale 网络集成 | BSD-3 |
| `github.com/gorilla/websocket` | WebSocket | BSD-2 |
| Android VpnService | VPN 隧道 | Apache 2.0 |
| Web Speech API | 语音输入 | 浏览器内置 |

全部兼容 MIT 协议，开源无任何许可问题。

---

## 4. Mac 端助手设计

### 4.1 对外命令

```bash
claude-phone-agent              # 启动服务（常驻后台）
claude-phone-agent key          # 生成一次性配对 Auth Key
claude-phone-agent status       # 查看当前连接的设备和活跃会话
```

### 4.2 内部组件

```
claude-phone-agent
│
├── tsnet 网络层
│   └── 加入 Tailscale 网络，主机名 = claude-mac
│       监听 :9876 (Tailscale 虚拟网络内)
│
├── WebSocket 服务器
│   └── 每台手机一个连接
│       协议: JSON over WebSocket
│
├── Session Manager
│   └── map[sessionId] → ClaudeSession (纯内存)
│       清理协程: 每 5 分钟扫描孤儿会话 → 超时 30 分钟 kill
│
└── 配置文件读取
    └── ~/.claude-phone/projects.yaml (工作目录列表)
```

### 4.3 会话内部

```
┌──────────────── ClaudeSession ────────────────┐
│  phone WS ──► translate ──► stdin  ──► claude │
│                                           process
│  phone WS ◄── translate ◄── stdout ◄── claude │
│
│  cancel() ──► SIGTERM(10s) ──► SIGKILL        │
└────────────────────────────────────────────────┘
```

每个会话 = 一个 `claude` 子进程 + 两个 translate goroutine（读/写管道的 JSON 翻译器）。

### 4.4 Claude CLI 启动参数

```bash
claude --print \
  --session-id <uuid> \
  --input-format stream-json \
  --output-format stream-json \
  --add-dir <工作目录1> \
  --add-dir <工作目录2> \
  --replay-user-messages
```

| 参数 | 作用 |
|------|------|
| `--print` | 非交互模式，stdin/stdout 走管道 |
| `--session-id` | 固定 session ID，支持 `--resume` 恢复 |
| `--input-format stream-json` | stdin 接受 JSON 行格式输入 |
| `--output-format stream-json` | stdout 输出 JSON 行格式（逐 token 流式） |
| `--add-dir` | 允许工具访问的目录，可传多个 |
| `--replay-user-messages` | 回显用户消息作为确认 |

### 4.5 消息流转（一条用户消息的完整路径）

```
手机发送:
  {"type":"text","content":"检查 QuoteService 并发安全性"}

       │ Mac 助手翻译为 claude stream-json 格式
       ▼
写入 claude stdin:
  {"type":"user","message":{"role":"user","content":[
    {"type":"text","text":"检查 QuoteService 并发安全性"}
  ]}}

       │ claude 处理
       ▼
claude stdout 输出 (逐行):
  {"type":"assistant","message":{"content":[{"type":"text","text":"让我先读取"}]}}
  {"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{...}}]}}
  {"type":"assistant","message":{"content":[{"type":"text","text":"发现死锁风险..."}]}}
  {"type":"result","subtype":"success"}

       │ Mac 助手翻译为手机协议格式
       ▼
WebSocket 发给手机:
  {"type":"thinking"}
  {"type":"token","content":"让我先读取"}
  {"type":"tool_use","tool":"Read","input":"{\"file_path\":\"...\"}"}
  {"type":"token","content":"发现死锁风险..."}
  {"type":"done"}
```

### 4.6 会话生命周期

```
        手机创建会话
             │
             ▼
        ┌─────────┐   手机断线 30 分钟    ┌──────────┐
        │ 活跃     │──────────────────►│  休眠     │
        │ claude   │                    │ 进程已kill │
        │ 进程运行  │                    │ 数据在磁盘  │
        └─────────┘                    └────┬─────┘
             │                              │
             │ 手机重连（无需操作）            │ 手机再次选择会话
             │                              │ claude --resume
             ▼                              ▼
        ┌─────────┐                    ┌──────────┐
        │ 活跃     │                    │ 唤醒中     │
        │ 恢复连接  │                    │ 启动中...  │
        └─────────┘                    └────┬─────┘
                                           │ ~1-2 秒
                                           ▼
                                      ┌─────────┐
                                      │ 活跃     │
                                      └─────────┘
```

**断线不立即杀进程**。你锁屏、切微信、地铁进隧道 → 回来看，会话还在。但如果 30 分钟不回来，进程被清理，下次点开会话时自动 `--resume` 恢复。

### 4.7 中断机制（cancel）

```
手机发送 {"type":"control","action":"cancel"}
  → 关闭 claude stdin (EOF)
  → 等待 claude 自行终止当前轮次（最多 10s）
  → 超时 → SIGTERM → 5s 后仍存活 → SIGKILL
  → 重新启动: claude --resume <sessionId>
  → 手机收到 {"type":"done"} 确认中断完成
```

### 4.8 权限管理

Claude Code 运行时会弹出权限确认（执行 Bash、写文件、网络请求等）。如果手机不能处理这些确认，会话就会卡死。

#### 4.8.1 三种权限模式

新建会话时可选：

```
新建会话 ──► 选择权限级别:
              ● 🟢 信任模式 (--permission-mode bypassPermissions)
                所有操作自动执行，零确认
              ○ 🟡 审阅模式 (--permission-mode acceptEdits)
                编辑自动通过，危险操作 (rm、curl 等) 需确认
              ○ 🔴 严格模式 (--permission-mode default)
                每步操作都需手动确认
```

大多数场景用 🟢 信任模式——本地开发信任 Claude 没问题。偶尔不确定的时候切到 🟡。

#### 4.8.2 权限请求的交互

当选了 🟡 或 🔴 模式，Claude 发起需要确认的操作时，手机上弹出确认卡片：

```
┌──────────────────────────────┐
│ ⚠️ Claude 需要确认             │
│                              │
│ 🔧 Bash 命令                  │
│ rm -rf /tmp/build-cache      │
│                              │
│ 清理构建缓存目录               │
│                              │
│  [✅ 允许]   [❌ 拒绝]        │
│  [🔄 始终允许此类操作]         │
└──────────────────────────────┘
```

#### 4.8.3 确认流程

```
claude stdout 输出请求:
  {"type":"permission_request","request_id":"req001",...}

助手:
  1. 暂停写 claude stdin（不让新消息干扰）
  2. WebSocket 发给手机 → 弹出确认卡片
  3. 等待手机回复
     - 允许 → 写回 claude stdin: {"type":"permission_response","response":"allow"}
     - 拒绝 → 写回: {"type":"permission_response","response":"deny"}
  4. claude 恢复执行

安全兜底: 60 秒无响应 → 自动拒绝，不卡会话
```

#### 4.8.4 批量确认

如果 Claude 一次发起多个操作，合并成一张卡片：

```
┌──────────────────────────────┐
│ ⚠️ Claude 需要执行 3 个操作    │
│                              │
│ 🔧 mkdir -p /tmp/output      │
│ 📝 写入 src/Result.java      │
│ 🔧 ./gradlew test            │
│                              │
│  [✅ 全部允许]  [❌ 全部拒绝]  │
│  [逐个确认 ▸]                │
└──────────────────────────────┘
```

#### 4.8.5 协议扩展

```json
// Mac → 手机: 权限请求
{"type":"permission_request","requestId":"req001","tool":"Bash","command":"rm -rf /tmp/cache","description":"清理构建缓存"}

// Mac → 手机: 批量权限请求
{"type":"permission_batch","batchId":"batch001","requests":[
  {"requestId":"req001","tool":"Bash","command":"mkdir -p /tmp/output"},
  {"requestId":"req002","tool":"Write","path":"src/Result.java"},
  {"requestId":"req003","tool":"Bash","command":"./gradlew test"}
]}

// 手机 → Mac: 单个回复
{"type":"permission_response","requestId":"req001","response":"allow"}

// 手机 → Mac: 单个拒绝 (可选原因)
{"type":"permission_response","requestId":"req001","response":"deny","reason":"不想删缓存"}

// 手机 → Mac: 批量回复
{"type":"permission_batch_response","batchId":"batch001","responses":[
  {"requestId":"req001","response":"allow"},
  {"requestId":"req002","response":"allow"},
  {"requestId":"req003","response":"deny","reason":"手动运行测试"}
]}

// 手机 → Mac: 记忆决策 (后续同类操作自动允许)
{"type":"permission_rule","tool":"Bash","pattern":"/tmp/*","response":"allow"}
```

### 4.9 健康监控与死锁处理

Claude 可能在运行中假死（API 超时、子 agent 卡住、死循环等）。助手必须能感知并处理。

#### 4.9.1 三条心跳线

助手对每个 claude 进程维护三个时间戳：

```
ClaudeSession {
    ...
    lastOutputTime   time.Time  // 任何 stdout 输出 (thinking/tool_use)
    lastTokenTime    time.Time  // 最后一次文本 token
    toolStartTime    time.Time  // 当前工具开始时间
    subAgents        map[string]*SubAgentStatus
}
```

一个独立的健康检查 goroutine，每 30 秒扫描所有活跃会话：

| 状态 | 判定条件 | 含义 |
|------|---------|------|
| 🟢 正常 | `now - lastOutputTime < 60s` | claude 在活动 |
| 🟡 可能卡死 | `now - lastOutputTime > 2min` | 长时间无输出 |
| 🔴 确认卡死 | `now - lastOutputTime > 5min` | 基本确认已挂 |
| 🔧 工具超时 | `now - toolStartTime > 3min` | 某命令执行过久 |
| 👻 子 agent 丢失 | 子 agent 创建后 3min 无任何事件 | 子 agent 可能挂了 |

#### 4.9.2 状态推送

状态变化时主动通知手机：

```json
{"type":"health","sessionId":"abc","status":"stale","idleSeconds":180,"detail":"已 3 分钟无响应"}
{"type":"health","sessionId":"abc","status":"tool_stuck","toolName":"Bash","idleSeconds":300,"detail":"命令已执行 5 分钟"}
{"type":"health","sessionId":"abc","status":"subagent_lost","agentName":"code-reviewer","idleSeconds":240}
```

#### 4.9.3 手机上的交互

```
claude 卡死时，聊天界面底部出现状态条：

🟡 Claude 已 3 分钟无响应，可能卡死
   可能原因: 子 agent 超时 / API 挂起 / 死循环
   [⏳ 继续等待]  [🔪 强制中断]

点"强制中断":
  → 助手发 SIGTERM → 5s → SIGKILL
  → 自动 claude --resume 恢复
  → 手机确认: "已中断并恢复，请继续对话"
```

#### 4.9.4 子 Agent 监控

从 claude stdout 解析子 agent 事件，助手维护清单：

```
subAgents:
  agent-001: 创建于 15:30, 最后事件 15:32, 状态: 运行中
  agent-002: 创建于 15:28, 最后事件 15:35, 状态: idle (已完成)
```

子 agent 超时时，助手注入系统消息提醒主 agent：

```json
// 写入 claude stdin
{"type":"system","message":"子 agent 'code-reviewer' 已 4 分钟无响应。请检查或重试。"}
```

#### 4.9.5 分级处理策略

| 状态 | 自动处理 | 通知用户 |
|------|---------|---------|
| 🟡 2min 无输出 | 无 | 轻提醒（小字状态条） |
| 🔴 5min 无输出 | 无（等用户决策） | 强提醒（弹窗 + 推送通知） |
| 🔧 工具 > 3min | 无（可能是大文件操作） | 状态条告知 |
| 🔧 工具 > 10min | 自动 SIGTERM 该工具 | 通知 + 自动继续 |
| 👻 子 agent > 3min | 注入提醒给主 agent | 状态条告知 |
| 👻 子 agent > 8min | 自动通知主 agent 重试 | 通知 |

#### 4.9.6 协议扩展

```json
// Mac → 手机: 健康状态
{"type":"health","sessionId":"abc","status":"stale|tool_stuck|subagent_lost|normal",
 "idleSeconds":180,"toolName":"Bash","agentName":"code-reviewer","detail":"..."}

// 手机 → Mac: 用户处置
{"type":"control","action":"force_kill","sessionId":"abc"}
{"type":"control","action":"wait_longer","sessionId":"abc"}  // 重置计时器，再等 5 分钟
```

#### 4.9.7 Mac 资源仪表盘（手机设置页）

```
┌──────────────────────────────┐
│  Mac 资源状态                │
│                              │
│  claude 进程: 3 个运行中      │
│  内存占用: 856 MB / 16 GB    │
│  CPU: 12%                    │
│  磁盘: 234 GB 可用           │
│                              │
│  子 agent 活动: 2 个活跃     │
└──────────────────────────────┘
```

### 4.10 启动与自启

```xml
<!-- ~/Library/LaunchAgents/com.claude.phone-agent.plist -->
<plist>
<dict>
    <key>Label</key>
    <string>com.claude.phone-agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/claude-phone-agent</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>EnvironmentVariables</key>
    <dict>
        <key>TS_AUTHKEY</key>
        <string>tskey-auth-permanent-xxx</string>
        <key>HOME</key>
        <string>/Users/binyangbin</string>
    </dict>
</dict>
</plist>
```

### 4.11 工作目录配置

```yaml
# ~/.claude-phone/projects.yaml
projects:
  - name: "开放平台"
    path: /Users/binyangbin/insurance-project/insurance-open-platform
  - name: "网关 SDK"
    path: /Users/binyangbin/insurance-project/insurance-gateway-executor
  - name: "保险大仓 (全部项目)"
    paths:
      - /Users/binyangbin/insurance-project
      - /Users/binyangbin/develop
```

手机新建会话时从预设列表中选择工作目录。可以选单项目、父目录、或多目录组合。助手不关心路径层级——原样传给 `claude --add-dir`。

---

## 5. 通信协议

### 5.1 手机 → Mac

```json
// 新建会话 (附带工作目录)
{"type":"control","action":"create_session","name":"车险理赔联调","workingDir":"/path/to/project"}

// 选择已有会话
{"type":"control","action":"select_session","sessionId":"550e8400-..."}

// 停止会话 (kill claude 进程 + 从 map 移除)
{"type":"control","action":"stop_session","sessionId":"550e8400-..."}

// 列出所有会话
{"type":"control","action":"list_sessions"}

// 获取可用工作目录列表
{"type":"control","action":"list_projects"}

// 发送文本消息 (当前选中会话)
{"type":"text","content":"帮我检查 XxxService 的并发问题"}

// 语音消息 (已由 Android 端识别为文字)
{"type":"voice","content":"语音识别后的文字"}

// 中断当前响应
{"type":"control","action":"cancel"}

// 权限回复 (单个)
{"type":"permission_response","requestId":"req001","response":"allow"}

// 权限回复 (拒绝 + 原因)
{"type":"permission_response","requestId":"req001","response":"deny","reason":"不想删缓存"}

// 权限回复 (批量)
{"type":"permission_batch_response","batchId":"batch001","responses":[
  {"requestId":"req001","response":"allow"},
  {"requestId":"req003","response":"deny"}
]}

// 记忆决策 (后续同类操作自动处理)
{"type":"permission_rule","tool":"Bash","pattern":"/tmp/*","response":"allow"}

// 心跳
{"type":"control","action":"ping"}

// Git 操作 (V1)
{"type":"control","action":"git","gitAction":"status|diff|log","sessionId":"abc"}
```

### 5.2 Mac → 手机

```json
// 工作目录列表
{"type":"project_list","projects":[{"name":"开放平台","path":"/..."},{"name":"保险大仓","paths":["...","..."]}]}

// 会话列表
{"type":"session_list","sessions":[
  {"sessionId":"...","name":"车险理赔联调","status":"active","createdAt":1720000000},
  {"sessionId":"...","name":"产品配置","status":"sleeping","createdAt":1720000001}
]}

// 会话创建成功
{"type":"session_created","sessionId":"550e8400-...","name":"车险理赔联调","cwd":"/path/to/project"}

// 会话已选中 (进入聊天)
{"type":"session_selected","sessionId":"550e8400-...","name":"车险理赔联调"}

// 会话已停止
{"type":"session_stopped","sessionId":"550e8400-..."}

// 流式响应 - 开始思考
{"type":"thinking"}

// 流式响应 - 文本增量
{"type":"token","content":"部分文本"}

// 流式响应 - 工具调用通知
{"type":"tool_use","tool":"Bash","input":"ls -la"}

// 流式响应 - 完成
{"type":"done"}

// 错误
{"type":"error","message":"错误描述"}

// 权限请求 (单个操作)
{"type":"permission_request","requestId":"req001","tool":"Bash","command":"rm -rf /tmp/cache","description":"清理构建缓存"}

// 权限请求 (批量)
{"type":"permission_batch","batchId":"batch001","requests":[
  {"requestId":"req001","tool":"Bash","command":"mkdir -p /tmp/output"},
  {"requestId":"req002","tool":"Write","path":"src/Result.java"},
  {"requestId":"req003","tool":"Bash","command":"./gradlew test"}
]}

// 心跳响应
{"type":"pong"}

// 健康状态推送
{"type":"health","sessionId":"abc","status":"stale","idleSeconds":180,"detail":"已 3 分钟无响应"}

// 任务完成通知
{"type":"notification","sessionId":"abc","sessionName":"车险联调","summary":"重构完成","actionCount":3,"duration":"6m"}

// 命令模板列表
{"type":"templates","templates":[{"label":"🔨 跑测试","prompt":"运行全部单元测试..."}]}

// Git 操作结果
{"type":"git_result","action":"status","result":"On branch pre\n..."}
```

---

## 6. 手机客户端设计

### 6.1 架构

```
APK
├── Kotlin Shell (~50 行)
│   ├── MainActivity: WebView + VpnService + 权限请求
│   └── JS Bridge: Go 核心 ↔ WebView 消息转发
│
├── Go 核心 (gomobile → .aar)
│   ├── tsnet 客户端: 加入 Tailscale 网络
│   ├── WebSocket 客户端: 连接 claude-mac:9876
│   └── 协议处理: 收/发 JSON 消息，通过 JS Bridge 与 UI 通信
│
└── Chat UI (WebView, HTML/CSS/JS)
    ├── 会话列表 (首页)
    ├── 聊天界面 (消息气泡 + 流式渲染 + 工具调用展示)
    └── 设置 (Mac 连接管理)
```

### 6.2 三屏结构

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────┐
│  会话列表      │     │  聊天界面         │     │  设置         │
│  (首页)       │     │                  │     │              │
│              │     │  ┌────────────┐  │     │ Mac 连接状态  │
│ + 新建会话    │     │  │ 用户消息     │  │     │              │
│              │     │  │ (右,蓝)     │  │     │ 工作目录管理  │
│ ┌──────────┐ │     │  └────────────┘  │     │              │
│ │ 车险联调  │ │     │                  │     │ 关于          │
│ │ 活跃 · 2h│─┤────→│  ┌────────────┐  │     │              │
│ └──────────┘ │     │  │ Claude 回复  │  │     │              │
│ ┌──────────┐ │     │  │ (左,灰)     │  │     │              │
│ │ 产品配置  │ │     │  │ 正在流式输入▌│  │     │              │
│ │ 休眠 · 1d│ │     │  └────────────┘  │     │              │
│ └──────────┘ │     │                  │     │              │
│              │     │  🔧 Bash: ls     │     │              │
│              │     │                  │     │              │
│              │     │  [🎤] [___] [➤] │     │              │
└──────────────┘     └──────────────────┘     └──────────────┘
```

- **首页**：会话列表。每个会话显示名字 + 状态 + 时间。左滑/长按停止。点 "+" 新建。
- **聊天界面**：进入一个会话。消息气泡 + 流式输入 + 语音按钮。
- **设置**：偶尔访问，管理连接和工作目录。

### 6.3 新建会话流程

```
┌──────────────────────────┐
│  新建会话                 │
│                          │
│  名称：[_______________] │
│                          │
│  工作目录：               │
│  ┌────────────────────┐  │
│  │ ● 开放平台           │  │
│  │ ○ 网关 SDK          │  │
│  │ ○ 保险大仓 (父目录)   │  │
│  └────────────────────┘  │
│                          │
│  权限级别：               │
│  ● 🟢 信任模式 (推荐)     │
│  ○ 🟡 审阅模式            │
│  ○ 🔴 严格模式            │
│                          │
│        [ 创 建 ]         │
└──────────────────────────┘
```

### 6.4 聊天界面状态

| Claude 状态 | UI 展示 |
|------------|---------|
| 正在思考 | 消息列表底部出现闪烁光标 `▌` |
| 流式输出 | 文本逐 token 追加到当前气泡，光标持续闪烁 |
| 工具调用 | 系统消息样式（居中、小字），折叠显示，可展开 |
| 响应完成 | 光标消失，输入框恢复可用 |
| 发送失败 | 消息气泡右侧红色 ❗，点击重试 |
| 权限请求 | 聊天界面弹出确认卡片，展示操作详情 + [允许]/[拒绝] 按钮。60 秒无响应自动拒绝 |
| 断线 | 顶栏状态点变红，底部黄色提示"连接断开，正在重连..." |
| 重连成功 | 提示消失，状态点变绿，继续之前的对话 |

### 6.5 语音输入

- 使用 WebView 原生 `Web Speech API`，无需额外权限（除麦克风）
- **按住说 → 松手识别 → 文字出现在输入框 → 手动点发送**（可以修改识别结果）
- 识别为空 → toast "未检测到语音"
- 录音时 mic 按钮有脉冲圆环动画

### 6.6 首次配对

```
首次打开 App：
┌──────────────────────────┐
│                          │
│   欢迎使用 Claude Phone  │
│                          │
│  ┌────────────────────┐  │
│  │ tskey-auth-xxxx... │  │  ← 粘贴 Auth Key
│  └────────────────────┘  │
│                          │
│  如何获取：               │
│  Mac 端运行：             │
│  claude-phone-agent key  │
│  会生成一次性 Auth Key    │
│                          │
│  ┌────────────────────┐  │
│  │       连 接         │  │
│  └────────────────────┘  │
└──────────────────────────┘
```

流程：
1. Mac 终端跑 `claude-phone-agent key` → 输出一次性 Auth Key（1 小时过期）
2. 手机粘贴 Key → 点连接
3. tsnet 自动加入你的 Tailscale 网络
4. 手机通过主机名 `claude-mac` 发现 Mac
5. 连接成功 → 进入会话列表

不需要扫码（异步场景更灵活），不需要输入 IP（tsnet 自动解析主机名）。

---

## 7. 扩展功能规划

以下功能不在 V1 范围，但架构预留了扩展点。

---

### 7.1 任务完成推送通知

Claude 长时间任务完成后，手机收到推送。

**场景**：手机上问"帮我重构 XxxService"，Claude 要跑 5-10 分钟。你锁屏，过一会儿通知栏弹出"重构完成：修改了 3 个文件，测试全部通过"。

**实现**：助手检测到 claude 进程进入 idle 状态 → 提取最后一条 assistant 消息作为 summary → WebSocket 推送 `{"type":"notification","summary":"...","files":[...]}` → 手机发本地通知。

**协议**：
```json
// Mac → 手机
{"type":"notification","sessionId":"abc","sessionName":"车险联调","summary":"重构完成，修改了 3 个文件，测试全部通过",
 "actionCount":3,"duration":"6m"}
```

**设置**：手机端可按会话开关通知，默认开。

---

### 7.2 预设命令模板

减少手机打字，一键发送常用指令。

```
聊天界面底部固定一排快捷按钮：
[🔨 跑测试] [📦 构建] [🔍 Review] [📝 文档] [📋 Git Log]
```

**实现**：配置在 `~/.claude-phone/templates.yaml`：

```yaml
templates:
  - label: "🔨 跑测试"
    prompt: "运行当前项目的全部单元测试，报告失败情况"
  - label: "🔍 Code Review"
    prompt: "Review 当前分支的改动，找出潜在 bug 和代码质量问题"
  - label: "📝 生成文档"
    prompt: "为当前项目的主要模块生成一份 README 文档"
  - label: "📋 Git Log"
    prompt: "查看最近 10 条 git log，总结主要改动"
```

**协议**：
```json
// Mac → 手机 (模板列表)
{"type":"templates","templates":[{"label":"🔨 跑测试","prompt":"运行当前项目的全部单元测试..."}]}
```

---

### 7.3 Git 快速操作

手机上快速查看 git 状态、diff、commit 历史。

```
设置页"Git 工具"入口:
┌──────────────────────────────┐
│  Git 快速操作                 │
│                              │
│  [📊 状态] [📝 Diff] [📋 Log] │
│                              │
│  当前分支: pre                │
│  Uncommitted: 3 files        │
│  上次 commit: fix(sign): ... │
└──────────────────────────────┘
```

**实现**：助手直接执行 git 命令（claude 不需要参与），结果原样返回手机。

**协议**：
```json
// 手机 → Mac
{"type":"control","action":"git","gitAction":"status","sessionId":"abc"}

// Mac → 手机
{"type":"git_result","action":"status","result":"On branch pre\nChanges not staged:\n..."}
```

---

### 7.4 轻量文件浏览器

手机上浏览 Mac 项目目录，点文件预览内容。比让 Claude "帮我看看 xxx 文件"更快。

```
┌──────────────────────────────┐
│  ← /src/main/java/service/   │
│                              │
│  📁 debug/                   │
│  📁 config/                  │
│  📄 DebugExecuteService.java │
│  📄 GatewayApplier.java      │
│  📄 DiffCalculator.java      │
└──────────────────────────────┘
         │ 点击文件
         ▼
┌──────────────────────────────┐
│  GatewayApplier.java (151行) │
│                              │
│  Line 1-50:                  │
│  package com.xiaoju...       │
│  import com.alibaba...       │
│  ...                         │
└──────────────────────────────┘
```

**实现**：助手收到 `browse_dir` 请求 → `os.ReadDir()` → 返回文件列表。手机端用 WebView 渲染，支持语法高亮。

**协议**：
```json
// 手机 → Mac
{"type":"control","action":"browse","path":"/src/main/java/com/..."}

// Mac → 手机
{"type":"browse_result","path":"/src/...","entries":[
  {"name":"debug","type":"dir"},
  {"name":"GatewayApplier.java","type":"file","size":4512}
]}

// 手机 → Mac
{"type":"control","action":"read_file","path":"/src/.../GatewayApplier.java"}

// Mac → 手机
{"type":"file_content","path":"/src/...","content":"package com...","language":"java","lines":151}
```

---

### 7.5 消息引用文件

聊天时 @ 文件——"帮我分析 @QuoteService.java 的这个方法"。类似 Slack 的文件引用。

```
输入框输入 @ → 弹出文件搜索对话框 → 选文件 → 自动插入文件引用
消息发送为: {"type":"text","content":"分析这个方法","mentions":["src/main/.../QuoteService.java"]}
```

**实现**：助手收到 mentions 后，自动读取引用文件内容，拼接进 claude 的 prompt：

```
用户在 <file path='QuoteService.java'> 中提到了这个文件。
[文件内容]
---
用户的问题: 分析这个方法
```

---

### 7.6 多 Mac 支持

手机上切换控制多台 Mac。

```
┌──────────────────────┐
│  选择 Mac             │
│                      │
│  ● 🖥 MacBook (本机) │
│  ○ 🖥 Mac Mini (办公室)│
│  ○ 🖥 Mac Studio    │
│                      │
│  [+ 添加 Mac]       │
└──────────────────────┘
```

**实现**：tsnet 天然支持——每台 Mac 用不同 hostname 加入同一个 Tailscale 网络。手机端维护一个 Mac 列表，切换时断开当前 WS，连新 Mac 的 WS。每台 Mac 各自管理自己的会话列表。

**协议**：
```json
// 手机 → Mac (当前连接)
{"type":"control","action":"list_macs"}

// Mac → 手机 (Mac 端维护同网络的 Mac 列表)
{"type":"mac_list","macs":[
  {"hostname":"claude-mac-macbook","name":"MacBook","status":"online"},
  {"hostname":"claude-mac-mini","name":"Mac Mini","status":"online"}
]}

// 手机 → 当前 Mac (请求切换)
{"type":"control","action":"switch_mac","hostname":"claude-mac-mini"}
```

---

### 7.7 定时任务

"每天早上 9 点跑一次 `./gradlew test`，有失败通知我"。

```
┌──────────────────────────────┐
│  定时任务                     │
│                              │
│  ┌────────────────────────┐  │
│  │ 每日测试    每天 09:00   │  │
│  │ 状态: 活跃              │  │
│  └────────────────────────┘  │
│  ┌────────────────────────┐  │
│  │ 周报        周五 17:00   │  │
│  │ 状态: 暂停              │  │
│  └────────────────────────┘  │
│                              │
│  [+ 新建定时任务]             │
└──────────────────────────────┘
```

**实现**：助手内嵌 cron 调度器（`robfig/cron`），配置文件持久化。每个定时任务绑定一个 Claude 会话或直接执行 shell 命令。

```yaml
# ~/.claude-phone/cron.yaml
jobs:
  - name: "每日测试"
    schedule: "0 9 * * *"
    workingDir: "/Users/binyangbin/insurance-project/insurance-open-platform"
    action: "claude"
    prompt: "运行 ./gradlew test，报告失败情况"
    notify: true
```

---

### 7.8 Git 事件触发

"当我 push 到 pre 分支时，自动跑一次集成测试"。

**实现**：助手启动一个轻量 HTTP 服务器监听 GitHub webhook。收到 push 事件 → 匹配规则 → 自动创建 Claude 会话执行预定义流程。

```yaml
# ~/.claude-phone/webhooks.yaml
webhooks:
  - name: "pre 分支自动检查"
    repo: "insurance-open-platform"
    branch: "pre"
    action: "claude"
    prompt: "拉取最新代码，运行 ./gradlew test，报告结果"
    notify: true
```

---

### 7.9 会话间上下文共享

"把车险联调会话里的死锁发现，同步到产品配置会话"。

**实现**：助手维护一个共享上下文存储（`~/.claude-phone/shared-context/`），用户手动选择将某人会话的关键发现写入共享存储。其他会话启动时自动注入共享上下文。

**手机端操作**：在 Claude 的某条回复上长按 → "共享到其他会话" → 选择目标会话 → 内容以系统消息注入目标会话。

---

### 7.10 功能优先级总览

| 优先级 | 功能 | 价值 | V1？ |
|--------|------|------|------|
| ⭐⭐⭐ | 任务完成推送通知 | 不用盯着屏幕等 Claude | V1 |
| ⭐⭐⭐ | 预设命令模板 | 减少手机打字 | V1 |
| ⭐⭐ | Git 状态/Diff 查看 | 快速了解项目状态 | V2 |
| ⭐⭐ | 定时任务 | 自动化例行检查 | V2 |
| ⭐⭐ | 多 Mac 支持 | 控制多台机器 | V2 |
| ⭐⭐ | 消息引用文件 | 聊天气泡里 @ 文件 | V2 |
| ⭐ | 文件浏览器 | 手机上翻代码 | V3 |
| ⭐ | Git 事件触发 | push 后自动检查 | V3 |
| ⭐ | 会话间共享上下文 | 多会话协同 | V3 |

---

## 8. iOS 支持规划

### 8.1 为什么不 V1 做 iOS

- iOS 分发需要签名/TestFlight/App Store，不能直接装 IPA
- iOS UI 需要用 SwiftUI 原生（WebView 体验差），Go 核心可复用但 UI 层要重写
- 语音输入需用 `SFSpeechRecognizer`（Swift 原生），不能像 Android 用 Web Speech API

**策略：V1 Android 先跑通，V2 扩展到 iOS。**

### 8.2 iOS 架构

```
┌──────────────────────────────────────┐
│  iOS App (Swift/SwiftUI)             │
│                                      │
│  ┌────────────────────────────────┐  │
│  │ Go 核心 (gomobile → .framework) │  │  ← ★ 和 Android 共享同一份 Go 代码
│  │  tsnet + WebSocket + 协议处理    │  │     pkg/protocol、pkg/session 完全复用
│  └────────────────────────────────┘  │
│                                      │
│  ┌────────────────────────────────┐  │
│  │ Chat UI (SwiftUI 原生)          │  │  ← iOS 原生 UI
│  │  - 会话列表 + 聊天界面           │  │
│  │  - 权限确认卡片                  │  │
│  └────────────────────────────────┘  │
│                                      │
│  ┌────────────────────────────────┐  │
│  │ 语音: SFSpeechRecognizer        │  │  ← iOS 原生语音识别
│  └────────────────────────────────┘  │
└──────────────────────────────────────┘
```

### 8.3 与 Android 端的差异

| | Android | iOS |
|---|---|---|
| Go 核心 | ✅ gomobile .aar | ✅ gomobile .framework（同一份代码） |
| UI 层 | WebView (HTML) | SwiftUI（不可复用） |
| 语音输入 | Web Speech API | SFSpeechRecognizer |
| 分发 | APK 直装 | TestFlight / App Store |

### 8.4 实现优先级

| 阶段 | 平台 | 内容 |
|------|------|------|
| V1 | Android | 全功能上线 |
| V2 | iOS | Go 核心复用 + SwiftUI UI 重写 + 语音适配 |
| V3 | 未来 | 如需统一体验 → 自建中继，三端共连 |

---

## 9. WebView vs 原生 UI 选择

选择 **WebView** 渲染聊天界面。

| | WebView | 原生 (Compose) |
|---|---|---|
| 开发速度 | 极快，AI 直接生成 HTML | 慢，手写 Kotlin |
| 聊天体验 | 完全一样 | 完全一样 |
| 语音输入 | Web Speech API (原生) | SpeechRecognizer |
| 代码体积 | HTML + CSS (~100KB) | Compose 依赖 (~5MB) |

聊天界面是 HTML/CSS 最强的领域。UI 由 AI 生成和维护，开发精力全部放在 Go 核心逻辑上。

---

## 10. 开源计划

- **仓库**：`github.com/yang-bin-free/claude-phone`
- **许可证**：MIT
- **发布**：GitHub Releases 提供预编译 APK + Mac 二进制
- **全部依赖兼容 MIT**（BSD-3/BSD-2/Apache 2.0），无 GPL 传染问题

### 开源合规清单

| 事项 | 说明 |
|------|------|
| 项目根放 `LICENSE` 文件 | MIT License, Copyright (c) 2026 yang-bin-free |
| README 末尾注明依赖 | 列出 Tailscale tsnet (BSD-3)、Gorilla WebSocket (BSD-2)、Android VpnService (Apache 2.0) 及其许可证 |
| 复制他人源码时保留版权头 | 如果直接复制了 Tailscale 源码文件，文件头的原始 Copyright 声明不可删除 |
| go.mod 本身就是依赖声明 | BSD/Apache 不要求贴出许可证原文，但建议加 `THIRD_PARTY_LICENSES.md` |
| 不需要征求原作者许可 | BSD/Apache/MIT 都是宽松协议，不要求事先授权 |
| 可以商用 | 三个依赖均允许商用，无附加条件 |

---

## 11. 实现阶段

| 阶段 | 内容 | 预计 |
|------|------|------|
| Phase 1 | Mac 助手核心: `pkg/protocol` + `pkg/session` + tsnet + claude 进程管理 + websocat 测试 | 2-3 天 |
| Phase 2 | Mac 助手完整: 权限管理 + 健康监控 + 通知推送 + Git 工具 | 1-2 天 |
| Phase 3 | Android Go 核心 (gomobile) + Kotlin 壳 + WebView Chat UI | 2-3 天 |
| Phase 4 | Android 完整: 语音输入 + 会话管理 + 权限确认卡片 + 健康状态展示 | 1-2 天 |
| Phase 5 | 打磨: 模板按钮 + 重连/中断 + 错误处理 + 状态指示 | 1-2 天 |
| Phase 6 | 开源: README + CONTRIBUTING + 构建文档 + GitHub Release | 1 天 |
| Phase 7 | iOS (V2): Go 核心复用 + SwiftUI Chat UI + SFSpeechRecognizer | 3-4 天 |

---

## 12. 讨论记录

- 方案演化: SSH+Termius → ttyd → 自建云中继 → 全 Go + tsnet
- 网络层: Tailscale tsnet 嵌入，零外部 App 依赖
- 会话管理: 惰性唤醒、30 分钟断线超时、`--resume` 恢复
- 工作目录: 创建会话时从预设列表选择，支持单项目/父目录/多目录
- UI: WebView + AI 生成 HTML，Web Speech API 语音输入
- 配对: Auth Key 文本输入，不支持二维码扫描（异步场景不适用）
