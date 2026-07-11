# Claude Phone — 手机控制 Mac Claude Code

> 设计方案 v2 | 2026-07-08 | 阿彬 & 小罗

---

## 1. 目标

用手机远程控制 Mac 上的 Claude Code，进行开发协作。

**最终体验**：
- 手机：只安装 **1 个 App**（Android APK / iOS IPA）
- Mac：只运行 **1 个二进制**（GUI `claude-phone` 或无头 `claude-phone-agent`）+ Claude Code CLI
- 不需要装 Tailscale App、不需要装 SSH 客户端、不需要配置端口映射、不需要记忆 IP 地址
- **多台手机可同时连接同一台 Mac**，各自独立操作，也可共享会话

**核心能力**：
1. 创建 / 切换 / 停止会话
2. 流式查看 Claude 回复
3. 语音输入
4. 权限确认（审阅 / 严格模式）
5. 多设备并发访问

---

## 2. 架构

### 2.1 三端总览

```
┌─────────────────────────────────┐
│        Tailscale 协调服务器       │
│   (只做地址交换 + 密钥分发, 不传数据) │
└──────────┬──────────────────────┘
           │ WireGuard 加密
     ┌─────┼─────────────────────┐
     │     │                     │
     ▼     ▼                     ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│  Mac              │  │  Android         │  │  iOS 18          │
│                   │  │                  │  │                  │
│  claude-phone-    │  │  Claude Phone    │  │  Claude Phone    │
│  agent            │  │  APK             │  │  IPA             │
│  (纯 Go 二进制)    │  │                  │  │                  │
│                   │  │  ┌────────────┐  │  │  ┌────────────┐  │
│  ┌─────────────┐  │  │  │Kotlin ~200行│  │  │  │SwiftUI 原生 │  │
│  │tsnet.Server │  │  │  │ VpnService  │  │  │  │NetworkExt.  │  │
│  │Hostname:    │  │  │  │ JS Bridge   │  │  │  │             │  │
│  │claude-mac   │  │  │  └─────┬──────┘  │  │  └─────┬──────┘  │
│  └──────┬──────┘  │  │        │         │  │        │         │
│         │         │  │  ┌─────▼──────┐  │  │  ┌─────▼──────┐  │
│  ┌──────▼──────┐  │  │  │Go 核心     │  │  │  │Go 核心     │  │
│  │WebSocket    │  │  │  │(gomobile→  │  │  │  │(gomobile→  │  │
│  │Server :9876 │  │  │  │ .aar)      │  │  │  │ .framework)│  │
│  └──────┬──────┘  │  │  │            │  │  │  │            │  │
│         │         │  │  │ Tailscale  │  │  │  │ Tailscale  │  │
│  ┌──────▼──────┐  │  │  │ 引擎       │  │  │  │ 引擎       │  │
│  │Session      │◄►┼──│  │ + WS Clt   │  │  │  │ + WS Clt   │  │
│  │Manager      │  │  │  │ + 协议处理  │  │  │  │ + 协议处理  │  │
│  └─────────────┘  │  │  └────────────┘  │  │  └────────────┘  │
│                   │  │                  │  │                  │
│  ┌─────────────┐  │  │  ┌────────────┐  │  │  ┌────────────┐  │
│  │claude --print│  │  │  │Chat UI     │  │  │  │Chat UI     │  │
│  │子进程管理     │  │  │  │WebView     │  │  │  │SwiftUI     │  │
│  └─────────────┘  │  │  │+Web Speech │  │  │  │+SpeechRec. │  │
│                   │  │  └────────────┘  │  │  └────────────┘  │
└──────────────────┘  └──────────────────┘  └──────────────────┘
```

### 2.2 三端差异

| | Mac | Android (V1) | iOS (V2) |
|---|---|---|---|
| **Go 核心** | 原生 Go 二进制 | gomobile → .aar | gomobile → .framework |
| **共享代码** | `pkg/protocol` + `pkg/session` | 同左 | 同左 |
| **Tailscale 接入** | `tsnet.Server{}` | VpnService + `ipnlocal.LocalBackend` + tun fd | NetworkExtension + `ipnlocal.LocalBackend` + tun fd |
| **UI 层** | — | WebView (HTML/CSS/JS) | SwiftUI 原生 |
| **语音输入** | — | Web Speech API | SFSpeechRecognizer |
| **构建产出** | 单二进制 | APK | IPA |
| **分发** | GitHub Release | GitHub Release / 直装 | TestFlight / App Store |

### 2.3 为什么全 Go

- Mac 端用 `tsnet` 几行代码加入 Tailscale 网络
- 手机端用 `gomobile` 编译为 .aar / .framework，三端共享 `pkg/protocol`、`pkg/session`
- 一个语言、一个工具链、零外部运行时依赖

### 2.4 网络层

#### Mac 端：tsnet（3 行加入网络）

```go
s := &tsnet.Server{Hostname: "claude-mac", Dir: "~/.claude-phone/tsnet-state"}
ln, _ := s.Listen("tcp", ":9876")
```

tsnet 自动处理 NAT 穿透、WireGuard 加密、密钥交换。`Dir` 持久化状态目录，重启后不需要新 Auth Key。

#### 手机端：Tailscale 引擎 + tun fd

tsnet 不支持 Android/iOS（[Issue #1748](https://github.com/tailscale/tailscale/issues/1748)），采用 Tailscale 官方移动端架构：

```
Kotlin/Swift 层                     Go 核心 (gomobile)
┌──────────────────┐               ┌──────────────────────┐
│ VpnService /     │   tun fd (int) │ ipnlocal.LocalBackend │
│ NetworkExtension ├──────────────►│ wireguard-go/tun     │
│                  │               │ netstack (userspace)  │
│ protect(fd) ◄────┼───────────────┤ socket 保护回调       │
└──────────────────┘               └──────────────────────┘
```

实现路径：fork `tailscale-android/libtailscale/`，精简保留 `backend.go` + `net.go` + `vpnfacade.go` + `interfaces.go`，新增 WebSocket Client + 协议层。

---

## 3. 项目结构

```
claude-phone/                       ← github.com/yang-bin-free/claude-phone
├── README.md
├── LICENSE (MIT)
├── go.mod
│
├── cmd/
│   ├── mac-agent/main.go           ← Mac 无头 Agent
│   └── mac-app/main.go             ← Mac 原生 WebView 桌面入口
│
├── pkg/
│   ├── androidlib/androidlib.go    ← Android Go 核心（gomobile → .aar）
│   ├── adminproto/messages.go      ← 本地管理协议
│   ├── desktop/                    ← 桌面 HTTP host + 原生 WebView
│   ├── engine/                     ← WS/tsnet/会话/管理 API
│   ├── protocol/messages.go        ← ★ 三端共享: JSON 消息定义 + 错误码
│   └── session/manager.go          ← ★ 三端共享: 会话管理
│
├── android/                        ← Android 壳
│   └── app/src/main/
│       ├── java/.../MainActivity.kt  ← ~200 行 Kotlin (VpnService + JS Bridge)
│       └── AndroidManifest.xml
│
├── ios/                            ← iOS 18 SwiftUI + Packet Tunnel 客户端
│   └── ClaudePhone/
│       ├── App.swift                ← 入口
│       ├── ChatView.swift           ← SwiftUI 聊天界面
│       └── NetworkExtension/        ← NEPacketTunnelProvider + tun fd
│
├── web/chat/                       ← 聊天 UI (AI 生成 + 维护)
│   ├── index.html
│   ├── core.css
│   ├── desktop.css
│   └── chat.js
├── web/admin/                      ← Mac 本地管理与诊断 UI
│
└── scripts/build.sh                ← gomobile bind + gradle assemble
```

### Android 构建入口

Android 工程固定使用 Gradle Wrapper 8.1 和 JDK 17+，不要直接运行全局 `gradle`，否则可能命中本机旧版 Gradle/JDK。

```bash
cd android
./build-android.sh assembleDebug
```

`build-android.sh` 会优先选择 Homebrew 的 `openjdk@17`，再调用项目内 `./gradlew`。
默认复用已有 `build/claudelib.aar`；需要重建 Go AAR 时执行：

```bash
REBUILD_AAR=1 ./build-android.sh assembleDebug
```

`REBUILD_AAR=1` 会调用 `scripts/build-android-aar.sh`。该脚本包含一个很窄的
gomobile 兼容绕过：当前 x/mobile 的 `gomobile bind` 会在临时 ABI 目录写入
0 字节 `go.mod`，Go 1.26 会拒绝这个文件。脚本只在 gomobile 临时目录中拦截
`go mod tidy`，补一个临时合法 `go.mod` 并 `replace` 回本仓库；普通 `go build`
/ `go test` 不受影响。等 x/mobile 修复或项目改用 patched gomobile 后可以删除
这段绕过。

Android 构建还会执行 `syncWebAssets`，把根目录 `web/chat` 与 `web/admin` 的
共享前端同步进 APK。App 首次启动显示 Mac 地址、Tailscale Auth Key 和可选
Control URL 配置；VPN 授权后启动内嵌 Go 引擎，并用系统 WebView 打开聊天页。
Mac 地址和 Control URL 存入应用私有配置，Auth Key 不持久化；Tailscale 登录
状态由 Go 核心保存在应用私有目录。

### Mac Agent 网络模式

默认只监听本机回环地址。P0c 跨网络联调时，通过持久化状态目录启用内嵌
tsnet；Auth Key 建议只通过环境变量传入，避免进入 shell 历史和仓库：

```bash
TS_AUTHKEY=tskey-auth-... claude-phone-agent serve \
  --addr :9876 \
  --tsnet-dir "$HOME/Library/Application Support/ClaudePhone/tsnet" \
  --tsnet-hostname claude-mac
```

使用 Headscale 等兼容控制服务器时，再传
`--tsnet-control-url https://headscale.example.com`。首次注册成功后，状态会保存在
`--tsnet-dir`，后续启动无需再次提供 Auth Key。

### Mac 桌面客户端

开发构建与启动：

```bash
go build -o claude-phone ./cmd/mac-app
./claude-phone --desktop-addr 127.0.0.1:9877
```

桌面客户端在进程内启动引擎、本地 HTTP/WebSocket 服务与 Cocoa/WebKit 原生窗口。
管理 API 同时校验回环来源和 256-bit 进程随机 token；token 仅通过 URL fragment
交给 WebView，不会进入 HTTP 请求或日志。原生窗口、菜单栏常驻和“关窗只隐藏”
生命周期已实现。无 GUI 环境继续使用 `claude-phone-agent`。

构建可分发目录（未签名、未公证）：

```bash
make android-apk
VERSION=0.1.0-dev make release
```

产物位于 `build/release/`，包含 Mac `.app` ZIP、Android APK、许可证文件和
`SHA256SUMS`。正式公开发布前仍需使用发布者自己的 Apple Developer ID 完成
codesign/notarization，并使用 Android release keystore 生成 release APK/AAB。

从仓库根目录验证发布包：

```bash
shasum -a 256 -c build/release/SHA256SUMS
```

Mac App 开机自启可通过当前可执行文件管理：

```bash
claude-phone autostart install
claude-phone autostart status
claude-phone autostart uninstall
```

### 依赖与许可证

| 依赖 | 用途 | 许可证 |
|------|------|--------|
| `tailscale.com/tsnet` | Mac 端 Tailscale 网络集成 | BSD-3 |
| `tailscale.com/ipnlocal` 等 | 手机端 Tailscale 引擎 | BSD-3 |
| `github.com/gorilla/websocket` | WebSocket | BSD-2 |
| Android VpnService | Android VPN 隧道 | Apache 2.0 |
| iOS NetworkExtension | iOS VPN 隧道 | 系统框架 API（无分发限制） |
| Web Speech API | Android 语音输入 | 浏览器内置 |
| SFSpeechRecognizer | iOS 语音输入 (V2) | 系统框架 API（无分发限制） |

全部兼容 MIT 协议，无 GPL 传染问题。

---

## 4. Mac 端设计

### 4.1 对外命令

```bash
claude-phone-agent              # 启动服务（常驻后台）
claude-phone-agent key --name Pixel  # 生成并持久化应用层 Device Token
claude-phone-agent status       # 查看连接的设备、活跃会话、资源状态（HTTP /status）
```

### 4.2 内部组件

```
claude-phone-agent
│
├── tsnet 网络层
│   └── 加入 Tailscale 网络 (hostname=claude-mac, 监听 :9876)
│
├── WebSocket 服务器
│   └── 多台手机同时连接 (device token 鉴权)
│       每台手机一个 WS 连接，协议: JSON over WebSocket
│
├── Session Manager
│   └── map[sessionId] → ClaudeSession (纯内存)
│       每个 session: owner + subscribers[] + claude 子进程
│       清理协程: 每 5 分钟扫描孤儿会话 → 超时 30 分钟 kill
│
├── 配置热加载（安全字段轮询）
│   ├── ~/.claude-phone/projects.yaml    (工作目录)
│   └── ~/.claude-phone/templates.yaml   (命令模板)
│
└── caffeinate 管理
    └── 有活跃会话时阻止 Mac 睡眠，无活跃会话时允许正常睡眠
```

### 4.3 多设备并发

Mac 端支持多台手机同时连接。典型场景：个人手机 + 工作手机同时在线。

```
┌──────────┐     ┌──────────┐
│ Pixel 8   │     │ iPhone   │
│ (device-A)│     │ (device-B)│
└─────┬────┘     └─────┬────┘
      │ WS                │ WS
      ▼                   ▼
┌────────────────────────────────┐
│  Mac                           │
│  session-1 → subscribers: [A]  │
│  session-2 → subscribers: [B]  │
│  session-3 → subscribers: [A,B]│  ← 共享会话
└────────────────────────────────┘
```

**规则**：
- 每个会话有 **owner**（创建者）和 **subscribers**（订阅者列表）
- 任何设备都能发消息，所有订阅者都收到流式输出
- 权限请求广播给所有订阅者，**先回复先生效**，60s 无响应自动拒绝
- 只有 owner 能停止会话，非 owner 只能离开（leave_session）
- 设备断线时从订阅列表移除，会话继续运行

### 4.4 会话内部

```
┌─────────────────── ClaudeSession ────────────────────┐
│                                                      │
│  ┌─ 订阅者 (多台手机) ◄► 双向 WS ──────────────┐   │
│  │  device-A WS ──┐                              │   │
│  │  device-B WS ──┤──► 消息合并 ──► translate    │   │
│  │  ...           ┘          │        ──► stdin   │   │
│  └───────────────────────────┼─────────────┐      │   │
│                              │             ▼      │   │
│                          translate     claude     │   │
│                              │         process    │   │
│                              ▼             │      │   │
│  ┌─ 输出扇出 ───────────────┤◄── stdout ──┘      │   │
│  │  device-A WS ◄──┐       │                     │   │
│  │  device-B WS ◄──┤── 广播                      │   │
│  │  ...           ┘                               │   │
│  └────────────────────────────────────────────────┘   │
│                                                      │
│  消息队列: per-session 全局 FIFO，所有设备共享          │
│  排队位置对所有订阅者可见 (queued 消息含 position)      │
│                                                      │
│  cancel() ──► close(stdin) ──► SIGTERM(10s) ──► SIGKILL │
└──────────────────────────────────────────────────────┘
```

### 4.5 Claude CLI 集成

```bash
claude --print \
  --session-id <uuid> \
  --input-format stream-json \
  --output-format stream-json \
  --add-dir <工作目录1> \
  --add-dir <工作目录2> \
  --permission-mode <bypassPermissions|acceptEdits|default> \
  --replay-user-messages
```

| 参数 | 作用 |
|------|------|
| `--print` | 非交互模式，stdin/stdout 走管道 |
| `--session-id` | 固定 ID，支持 `--resume` 恢复 |
| `--input-format stream-json` | stdin 接受 JSON 行格式 |
| `--output-format stream-json` | stdout 输出 JSON 行格式（逐 token 流式） |
| `--add-dir` | 允许工具访问的目录 |
| `--permission-mode` | 权限模式（信任/审阅/严格） |
| `--replay-user-messages` | 回显用户消息 |

**版本探测**：Mac 助手启动时执行 `<claude-bin> --version`，无法执行或无法解析版本时
拒绝启动；探测到的版本通过 hello 与本地诊断页展示。CLI 的 stream-json 格式由
translate 层和回归样例做兼容，不使用容易过期的硬编码主版本上限。

**并发会话上限**：默认最多 5 个同时运行的 claude 进程（可通过 `~/.claude-phone/config.yaml` 的 `maxConcurrentSessions` 调整）。超过上限时 `create_session` 返回错误 `SESSION_LIMIT_REACHED`。

### 4.6 消息流转

```
手机发送:
  {"type":"text","content":"检查并发安全性"}
       │ translate 层翻译为 claude stream-json
       ▼
写入 claude stdin:
  {"type":"user","message":{"role":"user","content":[
    {"type":"text","text":"检查并发安全性"}
  ]}}
       │ claude 处理
       ▼
claude stdout (逐行):
  {"type":"assistant","message":{"content":[{"type":"text","text":"让我先读取"}]}}
  {"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{...}}]}}
  {"type":"result","subtype":"success"}
       │ translate 层翻译 + 扇出广播
       ▼
WebSocket 发给所有订阅者:
  {"type":"thinking"}
  {"type":"token","content":"让我先读取"}
  {"type":"tool_use","tool":"Read","input":"..."}
  {"type":"done"}
```

**translate 层容错**：遇到非法 JSON 行 → 跳过 + WARN 日志；连续 N 行解析失败 → 通知手机 error + 暂停会话。遇到 `type` 未知的 JSON 行 → 保留原始 type，加 `unrecognized:true` 标记透传给手机，保证新版本 Claude 的新消息类型不会丢弃，手机端也能知道原始 type 名称：

```json
{"type":"some_new_type","unrecognized":true,"raw":"{\"type\":\"some_new_type\",...}"}
```

### 4.7 会话生命周期

```
      手机创建会话
           │
           ▼
      ┌─────────┐   所有订阅者断线 30 分钟    ┌──────────┐
      │ 活跃     │────────────────────────►│  休眠     │
      │ claude   │                         │ 进程已kill │
      │ 进程运行  │                         │ 数据在磁盘  │
      └─────────┘                         └────┬─────┘
           │                                    │
           │ 设备重连                             │ 设备选择会话
           │                                    │ claude --resume
           ▼                                    ▼
      ┌─────────┐                         ┌──────────┐
      │ 活跃     │                         │ 唤醒中     │
      │ 恢复连接  │                         │ 启动中...  │
      └─────────┘                         └────┬─────┘
                                               │ ~1-2s
                                               ▼
                                          ┌─────────┐
                                          │ 活跃     │
                                          └─────────┘
```

### 4.8 中断机制

```
手机发送 {"type":"control","action":"cancel"}
  → close(claude stdin) (EOF)
  → 等待 claude 自行终止当前轮次（最多 10s）
  → 超时 → SIGTERM → 5s 后仍存活 → SIGKILL
  → claude --resume 恢复
  → 所有订阅者收到 {"type":"done"}
```

### 4.9 权限管理

#### 三种权限模式（创建会话时选择）

```
● 🟢 信任模式 (bypassPermissions)  ← 推荐，零确认
○ 🟡 审阅模式 (acceptEdits)        ← 编辑自动过，危险操作需确认
○ 🔴 严格模式 (default)            ← 每步都需确认
```

#### 权限请求交互（审阅/严格模式）

```
┌──────────────────────────────┐
│ ⚠️ Claude 需要确认             │
│                              │
│ 🔧 Bash 命令                  │
│ rm -rf /tmp/build-cache      │
│                              │
│  [✅ 允许]   [❌ 拒绝]        │
│  [🔄 始终允许此类操作]         │
└──────────────────────────────┘
```

- 多设备场景：广播给所有订阅者，先回复先生效，其他设备收到 `permission_resolved` 自动关闭卡片
- 批量操作合并为一张卡片
- 60s 无响应自动拒绝
- "始终允许" → 记忆规则，后续同类操作自动处理
- 规则持久化到 `~/.claude-phone/permission-rules.json`，重启后仍生效
- 手机端 `设置 → 权限规则` 可查看 / 删除已记忆的规则

### 4.10 健康监控

对每个 claude 进程维护 `lastOutputTime`、`lastTokenTime`、`toolStartTime`、`subAgents`。独立 goroutine 每 30s 扫描：

| 状态 | 判定条件 | 自动处理 | 通知用户 |
|------|---------|---------|---------|
| 🟢 正常 | 有输出 < 60s | — | — |
| 🟡 可能卡死 | 无输出 > 2min | — | 状态条提醒 |
| 🔴 确认卡死 | 无输出 > 5min | — | 弹窗 + 推送 |
| 🔧 工具超时 | 工具 > 3min | — | 状态条 |
| 🔧 工具卡死 | 工具 > 10min | SIGTERM | 通知 |
| 👻 子 agent 丢失 | 无事件 > 3min | 注入提醒给主 agent | 状态条 |
| 👻 子 agent 卡死 | 无事件 > 8min | 通知主 agent 重试 | 通知 |

用户可点"强制中断" → SIGTERM → resume；或"继续等待" → 重置计时器。

### 4.11 Mac 睡眠策略

- **有活跃会话时**：引擎持有一个 `caffeinate -s` 进程阻止系统睡眠（显示器可关）
- **无活跃会话时**：允许正常睡眠
- **Mac 已睡眠时手机请求**：手机显示"Mac 不可用" + 重试按钮（每 30s ping）
- **Mac 唤醒后**：tsnet 自动重连，WebSocket 恢复，claude 进程若被 kill 则 `--resume`

| 场景 | 手机端展示 |
|------|-----------|
| Mac 在线 | 顶栏绿点 |
| Mac 睡眠/离线 | 红点 + "Mac 不可用" + [重试] |
| 重连中 | 黄点 + "正在重连..." |

### 4.12 消息历史

**Mac 端存储**（进程内串行追加）：

```
~/.claude-phone/sessions/<sessionId>/
├── meta.json          # name, cwd, createdAt, permissionMode, owner
└── messages.jsonl     # 每行一条 JSON
```

`messages.jsonl` 保留原始入站文本与 Claude stream-json 输出。进入会话后客户端通过
`load_history` 拉取最近 N 条并重放；默认 UI 请求最近 500 条，DOM 同样限制为最近
500 个消息节点。历史不自动删除。

### 4.13 断线重连

**手机端**：指数退避重连（1s → 2s → 4s → ... → 最大 30s），5 分钟仍失败则停止并显示[手动重试]。网络切换时立即重置退避。

**Mac 端**：WS 断开时不立即清理会话，让 claude 继续运行；30 分钟后仍无连接才清理孤儿会话。

### 4.14 Agent 崩溃恢复

Agent 自身崩溃（OOM / panic / 被 kill）时：

- claude 子进程可能变成孤儿（Agent 死了但 claude 还在跑）
- `messages.jsonl` + `meta.json` 是持久化的恢复数据源
- Agent 为每个自己拉起的 claude 子进程记录 PID + 启动时间戳，写入该会话的 `meta.json`

**恢复流程**：

1. Agent 启动时扫描 `~/.claude-phone/sessions/` 目录
2. 对有 `meta.json` 但没有活跃 claude 进程的会话标记为 `dormant`
3. 孤儿进程清理：**只按 `meta.json` 记录的 PID 精确匹配**（并校验启动时间戳防 PID 复用），确认是本 Agent 之前拉起的子进程才 SIGTERM——**绝不用 `ps aux | grep claude` 全局扫描**，避免误杀用户在同一台 Mac 上自己打开的 Claude Code 进程
4. 设备重连后可选择 `select_session` 恢复 dormant 会话（`claude --resume`）

### 4.15 并发消息

`claude --print` 一次处理一条消息，期间新消息排队：

- Claude 回复期间：输入框可用，新消息标记"排队中"（灰色气泡）
- 当前轮结束：自动发送队列中的下一条
- 用户可取消排队：长按 → "取消发送"

### 4.16 认证与安全

**两层认证**：

1. **Tailscale 网络层**：Mac tsnet 用 `Dir` 持久化状态，重启无需新 key；手机端通过 Auth Key 首次配对加入网络
2. **应用层**：WS 连接建立时验证 device token

```
配对流程:
  1. 用户在自己的 Tailscale 管理台生成 Auth Key，手机用它加入 tailnet
  2. Mac: claude-phone-agent key --name Pixel → 生成 dt_xxx Device Token
  3. 手机分别粘贴 Tailscale Auth Key 和 Device Token → 点连接
  4. Mac 将 Device Token 以 0600 权限持久化到 devices.yaml
  5. 后续连接: WS 第一条消息 {"type":"auth","deviceToken":"dt_xxx"}
```

**安全清单**：

| 项 | 说明 |
|----|------|
| WireGuard 加密 | 所有流量端到端加密 |
| Tailscale ACL | 限制只有 tag:claude-phone 可访问 claude-mac:9876 |
| Device token | 防止未授权设备连接 |
| 权限模式 | 信任/审阅/严格三档 |

### 4.17 工作目录与模板

```yaml
# ~/.claude-phone/projects.yaml
projects:
  - name: "开放平台"
    path: /Users/binyangbin/insurance-project/insurance-open-platform
    permission: default
  - name: "保险大仓"
    path: /Users/binyangbin/insurance-project
    permission: acceptEdits
```

```yaml
# ~/.claude-phone/templates.yaml
templates:
  - label: "🔨 跑测试"
    prompt: "运行当前项目的全部单元测试，报告失败情况"
  - label: "🔍 Code Review"
    prompt: "Review 当前分支的改动，找出潜在 bug"
```

`projects.yaml` 与 `templates.yaml` 每次请求时读取最新内容；`config.yaml` 每秒检查一次。
以下运行时字段可热更新，无需重启：

```yaml
defaultWorkingDir: /Users/me/project
defaultPermission: default
maxConcurrentSessions: 5
```

权限规则在 Mac 管理页增删，持久化到 `permission-rules.json`，并通过 Claude CLI
官方 `--allowedTools` 参数应用到新建或恢复的会话。

### 4.18 启动与自启

`claude-phone autostart install` 会生成用户级
`~/Library/LaunchAgents/com.claude.phone.plist` 并通过 `launchctl bootstrap` 加载；
卸载与状态查询使用同一命令的 `uninstall` / `status` 子命令。

tsnet 通过 `Dir` 持久化认证状态，不需要在 plist 里放 Auth Key。

---

## 5. 通信协议

### 5.1 手机 → Mac

```json
// 认证（WS 连接后第一条消息）
{"type":"auth","deviceToken":"dt_abc123...","deviceName":"Pixel 8"}

// 新建会话
{"type":"control","action":"create_session","name":"车险联调","workingDir":"/path","permissionMode":"bypassPermissions"}

// 选择会话（加入订阅）
{"type":"control","action":"select_session","sessionId":"550e8400-..."}

// 加入会话（非 owner 订阅）
{"type":"control","action":"join_session","sessionId":"550e8400-..."}

// 离开会话（取消订阅）
{"type":"control","action":"leave_session","sessionId":"550e8400-..."}

// 停止会话（仅 owner）
{"type":"control","action":"stop_session","sessionId":"550e8400-..."}

// 列出会话（分页）
{"type":"control","action":"list_sessions","limit":20,"offset":0}

// 获取工作目录列表
{"type":"control","action":"list_projects"}

// 发送文本
{"type":"text","content":"帮我检查并发问题"}

// 语音消息（已识别为文字）
{"type":"voice","content":"语音识别后的文字"}

// 中断当前响应
{"type":"control","action":"cancel"}

// 权限回复
{"type":"permission_response","requestId":"req001","response":"allow|deny","reason":"..."}

// 批量权限回复
{"type":"permission_batch_response","batchId":"batch001","responses":[
  {"requestId":"req001","response":"allow"},
  {"requestId":"req003","response":"deny","reason":"手动测试"}
]}

// 记忆决策
{"type":"permission_rule","tool":"Bash","pattern":"/tmp/*","response":"allow"}

// 请求历史消息
{"type":"control","action":"load_history","sessionId":"abc","limit":50,"beforeMsgId":"msg020"}

// 心跳
{"type":"control","action":"ping"}

// 强制中断卡死的会话（仅 owner）
{"type":"control","action":"force_kill","sessionId":"abc"}

// 继续等待（重置计时器）
{"type":"control","action":"wait_longer","sessionId":"abc"}
```

### 5.2 Mac → 手机

```json
// 连接建立
{"type":"hello","agentVersion":"0.1.0","claudeVersion":"1.0.45","protocolVersion":"1"}

// 工作目录列表
{"type":"project_list","projects":[{"name":"开放平台","path":"/..."},...]}

// 会话列表
{"type":"session_list","sessions":[
  {"sessionId":"abc","name":"车险联调","status":"active","owner":"device-A",
   "subscribers":["device-A","device-B"],"createdAt":1720000000}
]}

// 会话创建成功
{"type":"session_created","sessionId":"550e8400-...","name":"车险联调","cwd":"/path"}

// 会话已选中
{"type":"session_selected","sessionId":"550e8400-...","name":"车险联调","messageCount":42,"lastMsgId":"msg042"}

// 会话已停止
{"type":"session_stopped","sessionId":"550e8400-..."}

// 流式响应
{"type":"thinking"}
{"type":"token","content":"部分文本"}
{"type":"tool_use","tool":"Bash","input":"ls -la"}
{"type":"done"}

// 错误（含错误码）
{"type":"error","code":"SESSION_NOT_FOUND","message":"会话不存在"}

// 权限请求
{"type":"permission_request","requestId":"req001","tool":"Bash","command":"rm -rf /tmp/cache","description":"清理缓存"}

// 批量权限请求
{"type":"permission_batch","batchId":"batch001","requests":[...]}

// 权限已被其他设备处理
{"type":"permission_resolved","requestId":"req001","response":"allow","resolvedBy":"device-B"}

// 消息排队
{"type":"queued","msgId":"msg043","position":1}
{"type":"dequeued","msgId":"msg043"}

// 历史消息
{"type":"history","sessionId":"abc","messages":[
  {"msgId":"msg001","ts":"...","dir":"in","type":"text","content":"..."},
  {"msgId":"msg002","ts":"...","dir":"out","type":"token","content":"..."}
],"hasMore":true}

// 健康状态
{"type":"health","sessionId":"abc","status":"stale|tool_stuck|subagent_lost|normal",
 "idleSeconds":180,"detail":"..."}

// 任务完成通知
{"type":"notification","sessionId":"abc","sessionName":"车险联调","summary":"重构完成","actionCount":3,"duration":"6m"}

// 命令模板
{"type":"templates","templates":[{"label":"🔨 跑测试","prompt":"..."}]}

// 心跳
{"type":"pong"}
```

### 5.3 错误码

| Code | 含义 |
|------|------|
| `SESSION_NOT_FOUND` | 会话不存在 |
| `SESSION_NOT_OWNER` | 非 owner 尝试停止/强制中断会话 |
| `SESSION_LIMIT_REACHED` | 并发会话数达到上限 (默认 5) |
| `DEVICE_NOT_AUTHORIZED` | device token 无效 |
| `CLAUDE_NOT_FOUND` | claude CLI 未安装 |
| `CLAUDE_VERSION_MISMATCH` | claude 版本不兼容 |
| `TRANSLATE_ERROR` | translate 层解析失败 |
| `MAC_SLEEPING` | Mac 处于睡眠状态 |

---

## 6. 手机客户端设计

### 6.1 Android 架构

```
APK
├── Kotlin (~200 行)
│   ├── MainActivity: WebView + 权限请求
│   ├── IPNService: VpnService 子类 + tun fd 管理 + protect 回调
│   └── JS Bridge: Go 核心 ↔ WebView 消息转发
│
├── Go 核心 (gomobile → .aar)
│   ├── Tailscale 引擎: ipnlocal.LocalBackend + wireguard-go/tun + netstack
│   ├── WebSocket 客户端: 连接 claude-mac:9876
│   └── 协议处理: 收/发 JSON 消息，通过 JS Bridge 与 UI 通信
│
└── Chat UI (WebView, HTML/CSS/JS)
    ├── 会话列表 (首页)
    ├── 聊天界面 (消息气泡 + 流式渲染 + 工具调用展示)
    └── 设置 (Mac 连接管理)
```

**VpnService.protect() — 防路由环路**：

Android VpnService 创建的 tun 接口会拦截所有网络流量。如果 Go 核心创建的 socket（WebSocket 连接、Tailscale peer 通信等）也被 tun 拦截，就会形成路由环路——数据发出后又被自己的 tun 截回来，无限循环。

**protect 必须覆盖 Tailscale 引擎的所有 socket，不能只保护应用层的 WebSocket。** Tailscale 引擎内部会创建大量 socket：wireguard-go 的 UDP peer 通信、netstack 的 TCP 连接、magicsock 的 disco 包等。如果这些不 protect，同样会环路。

正确做法：protect 回调注入到 `wgengine` 的 dialer 接口层面，引擎创建的所有 socket 统一走 protect（参照 Tailscale 官方 Android 客户端的 `vpnfacade.go` 实现）：

```go
// Go 核心: protect 回调注册到 wgengine dialer（不是只包一个 net.Dial）
type VPNFacade struct {
    protect func(fd int) bool  // Kotlin → Go 的回调
}

// VPNFacade 实现 router.Router + dns.OSConfigurator
// wgengine 创建 socket 时，通过 dialer 接口调用 protect

func (v *VPNFacade) Dialer() *tlsdialer.Dialer {
    return &tlsdialer.Dialer{
        ProtectFunc: v.protect,  // ★ 所有引擎 socket 都走这里
    }
}
```

```kotlin
// Kotlin 侧: IPNService 中实现 protect
class IPNService : VpnService(), libtailscale.IPNService {
    override fun protect(fd: Int): Boolean {
        return protect(fd)  // VpnService.protect() 排除 socket 出 VPN 隧道
    }
}
```

**遗漏 protect 的后果**：连接卡死在路由环路，表现为"连接中..."永远不成功，且无任何错误输出。这是 Android VPN 开发最常见的坑。

### 6.2 iOS 18 架构

```
IPA
├── SwiftUI 原生 UI
│   ├── 会话列表 + 聊天界面 + 权限确认卡片
│   └── SFSpeechRecognizer 语音输入
│
└── Go 核心 (gomobile → .framework)  ← 和 Android 共享同一份代码
    ├── Tailscale 引擎 (NetworkExtension + tun fd)
    ├── WebSocket Client
    └── 协议处理 (pkg/protocol)
```

iOS 与 Android 保持相同业务能力和流程，使用 SwiftUI、Network Extension、
SFSpeechRecognizer 和 gomobile XCFramework 做平台原生实现。完整构建、签名和真机 VPN
验收需要 Xcode、iOS 18 SDK、Apple Developer Team 与 Network Extension entitlement；
详见 `ios/README.md`。

### 6.3 三屏结构

```
┌──────────────┐     ┌──────────────────┐     ┌──────────────┐
│  会话列表      │     │  聊天界面         │     │  设置         │
│  (首页)       │     │                  │     │              │
│ + 新建会话    │     │  用户消息 (右,蓝) │     │ Mac 连接状态  │
│              │     │                  │     │ 工作目录管理  │
│ 车险联调      │────→│  Claude 回复     │     │ 诊断信息     │
│ 活跃 · 2h    │     │  (左,灰) 流式▌   │     │ 关于          │
│              │     │                  │     │              │
│ 产品配置      │     │  🔧 Bash: ls     │     │              │
│ 休眠 · 1d    │     │                  │     │              │
│              │     │  [🎤] [___] [➤] │     │              │
└──────────────┘     └──────────────────┘     └──────────────┘
```

### 6.4 新建会话

```
┌──────────────────────────┐
│  新建会话                 │
│                          │
│  名称：[_______________] │
│                          │
│  工作目录：               │
│  ● 开放平台               │
│  ○ 网关 SDK              │
│  ○ 保险大仓 (父目录)     │
│                          │
│  权限级别：               │
│  ● 🟢 信任模式 (推荐)    │
│  ○ 🟡 审阅模式           │
│  ○ 🔴 严格模式           │
│                          │
│        [ 创 建 ]         │
└──────────────────────────┘
```

### 6.5 聊天界面状态

| Claude 状态 | UI 展示 |
|------------|---------|
| 正在思考 | 底部闪烁光标 `▌` |
| 流式输出 | 逐 token 追加，光标闪烁（用 `requestAnimationFrame` 节流） |
| 工具调用 | 居中小字，折叠显示，可展开 |
| 响应完成 | 光标消失，输入框恢复 |
| 权限请求 | 弹出确认卡片 |
| 断线 | 顶栏红点 + "正在重连..." |
| 排队消息 | 灰色气泡 + "等待中" |

### 6.6 语音输入

- **Android**：系统 `RecognizerIntent`（点麦克风 → 识别 → 文字回填输入框 → 手动发送）
- **iOS**：`SFSpeechRecognizer`

### 6.7 首次配对

```
┌──────────────────────────┐
│   欢迎使用 Claude Phone   │
│                          │
│  ┌────────────────────┐  │
│  │ tskey-auth-xxxx... │  │  ← Tailscale Auth Key
│  └────────────────────┘  │
│  ┌────────────────────┐  │
│  │ dt_xxxx...         │  │  ← Device Token
│  └────────────────────┘  │
│                          │
│  Device Token 获取方式:   │
│  agent key --name Pixel  │
│                          │
│  ┌────────────────────┐  │
│  │       连 接         │  │
│  └────────────────────┘  │
└──────────────────────────┘
```

### 6.8 WebView 性能

聊天场景对 WebView 有压力，V1 需要两个基本策略：

- **虚拟列表**：只渲染可视区域 ± buffer 的消息，避免长会话 DOM 膨胀
- **token 追加用 `requestAnimationFrame`**：避免每个 token 触发重排

超长代码块默认折叠（显示前 20 行 + "展开全部"）。

---

## 7. 实现阶段

| Phase | 内容 | 预计 |
|------|------|------|
| **P0a** | gomobile bind 最小 POC：HelloWorld Go → .aar → Kotlin 调用成功 | 0.5 天 |
| **P0b** | fork libtailscale，精简后 gomobile bind → .aar → VpnService + tun fd 传递成功 | 1 天 |
| **P0c** | Mac tsnet + Android Tailscale 引擎跨网络连接成功 | 0.5 天 |
| **P1** | Mac 助手核心：`pkg/protocol` + `pkg/session` + tsnet + claude 进程管理 + translate 层 + websocat 测试 | 2-3 天 |
| **P2** | Mac 助手完整：权限管理 + 健康监控 + 消息历史 + 断线重连 + caffeinate + 配置热加载 | 2 天 |
| **P2-GUI** | Mac 桌面客户端：管理 API + 共享聊天 UI + 原生 WebView + 菜单栏常驻 | 已实现，待实机 UI 验证 |
| **P3** | Android：Go 核心 + 前台 VpnService + WebView Chat UI + 语音 + 首次配对 | 已实现，待实体设备验证 |
| **P4** | 打磨：模板按钮 + 推送通知 + 错误处理 + 状态指示 + WebView 性能优化 | 2 天 |
| **P5** | 开源：README + CONTRIBUTING + 构建文档 + GitHub Release | 1 天 |
| **P6** | iOS 18：Go 核心 + Packet Tunnel + SwiftUI + SFSpeechRecognizer | 源码已实现，待 Xcode/签名/真机验收 |

每个 Phase 有明确的 pass/fail 标准，P0a/P0b/P0c 是整个项目风险最大的验证点。

---

## 8. 扩展功能（V2+）

| 优先级 | 功能 | 价值 | 版本 |
|--------|------|------|------|
| ⭐⭐ | Git 状态/Diff 查看 | 快速了解项目状态 | V2 |
| ⭐⭐ | 定时任务 (cron) | 自动化例行检查 | V2 |
| ⭐⭐ | 多 Mac 支持 | 控制多台机器 | V2 |
| ⭐⭐ | 消息引用文件 (@file) | 聊天气泡里引用文件内容 | V2 |
| ⭐ | 协议版本协商 | 版本兼容 | V2 |
| ⭐ | 暗黑模式 | 开发者体验 | V2 |
| ⭐ | 操作审计日志 | 安全合规 | V2 |
| ⭐ | 文件浏览器 | 手机上翻代码 | V3 |
| ⭐ | Git 事件触发 (webhook) | push 后自动检查 | V3 |
| ⭐ | 会话间共享上下文 | 多会话协同 | V3 |

---

## 9. 开源计划

- **仓库**：`github.com/yang-bin-free/claude-phone`
- **许可证**：MIT
- **发布**：GitHub Releases 提供预编译 APK + Mac 二进制

### 9.1 许可证兼容性分析

我们项目 MIT 许可证，依赖的全部是宽松协议，无 GPL 传染问题：

| 依赖 | 许可证 | 与 MIT 兼容？ | 说明 |
|------|--------|-------------|------|
| `tailscale.com/tsnet` | BSD-3 | ✅ | Mac 端使用，原始版权头保留即可 |
| `tailscale.com/ipnlocal` 等 | BSD-3 | ✅ | 手机端 libtailscale fork，**必须保留所有原始版权头** |
| `github.com/gorilla/websocket` | BSD-2 | ✅ | WebSocket 库 |
| `github.com/tailscale/wireguard-go` | MIT | ✅ | tun fd 集成用 |
| Android VpnService | Apache 2.0 | ✅ | 系统 API，不涉及源码分发 |
| iOS NetworkExtension | 系统框架 API | ✅ | 无分发限制，不涉及源码分发 |
| Web Speech API | 浏览器内置 | ✅ | — |
| SFSpeechRecognizer | 系统框架 API | ✅ | 无分发限制，不涉及源码分发 |

### 9.2 Fork Tailscale 源码的合规要求 ⚠️

这是合规风险最高的点。我们从 `tailscale/tailscale-android` fork 了 `libtailscale/` 目录（`backend.go`、`net.go`、`vpnfacade.go`、`interfaces.go` 等文件），这些文件原始许可证为 **BSD-3**（Tailscale Inc & AUTHORS）。

**必须做到**：

| 事项 | 说明 |
|------|------|
| **保留原始版权头** | 每个 fork 的 `.go` 文件**顶部 3 行版权注释不可删除或修改**。例如：`// Copyright (c) Tailscale Inc & contributors` + `// SPDX-License-Identifier: BSD-3-Clause` |
| **保留 SPDX 标识** | 不删除 `SPDX-License-Identifier` 行，自动化工具依赖它做许可证扫描 |
| **新增代码放新文件** | 我们自己新增的代码**放在独立的新文件里**，加 `// SPDX-License-Identifier: MIT`。**不要在 fork 的文件里混写两种许可证**——一个文件两种许可证会让自动化扫描工具和人类读者都困惑 |
| **NOTICE 文件** | BSD-3 要求在二进制分发时提供版权声明。项目根放 `NOTICE` 文件，列出所有 BSD 依赖的版权声明 |
| **go.mod 记录依赖** | `go.mod` 本身就是依赖声明，BSD/Apache 不要求贴出许可证原文 |

**严禁**：

- ❌ 把 Tailscale 的 BSD-3 文件改标为 MIT
- ❌ 删除或替换原始 `Copyright` 行
- ❌ 把 fork 代码和自写代码混在同一个文件标注不同许可证

### 9.3 二进制分发的合规

GitHub Release 发布 APK + Mac 二进制时：

| 事项 | 说明 |
|------|------|
| `LICENSE` 文件 | 项目根，MIT License 全文 |
| `NOTICE` 文件 | 项目根，列出 Tailscale BSD-3 版权声明（BSD 要求二进制分发时携带） |
| `THIRD_PARTY_LICENSES.md` | 列出所有依赖的许可证名称 + 版权声明，方便用户和自动化工具审查 |
| Release notes | 附上"本产品包含 Tailscale (BSD-3) 和 Gorilla WebSocket (BSD-2) 的代码" |

### 9.4 Tailscale Auth Key 使用合规

Tailscale 的 Auth Key 是通过 Tailscale 协调服务器完成设备认证的。开源项目中：

- **可以包含**：使用 tsnet / libtailscale 的 Go 代码（这是公开 API）
- **不可包含**：具体的 Auth Key 值（`tskey-auth-xxx`）放在源码或 plist 中——这属于用户个人凭据
- **README 说明**：引导用户自建 Tailscale 网络 + 自己生成 Auth Key，不要共享协调服务器的访问权限

### 9.5 开源合规检查清单

| 检查项 | 阶段 | 负责人 |
|--------|------|--------|
| 项目根 `LICENSE` (MIT) | P1 | — |
| 项目根 `NOTICE` (BSD-3 版权声明) | P1 | — |
| `THIRD_PARTY_LICENSES.md` | P1 | — |
| fork 文件原始版权头保留（CI 检查） | P3 | CI 脚本 |
| go.mod 依赖无 GPL | P1 | `go mod` 自动管理 |
| 源码中无硬编码 Auth Key | P1 | CI 脚本 |
| Release 附带合规声明 | P5 | — |

**CI 防回归**：建议在 CI 中加一步检查，确保 fork 的 Tailscale 文件头部包含 `SPDX-License-Identifier: BSD-3-Clause`，防止未来维护中误删版权头。

---

## 10. 方案演化

SSH+Termius → ttyd → 自建云中继 → **全 Go + Tailscale**

关键决策：
- **网络层**：Tailscale 嵌入，零外部 App 依赖；Mac 用 tsnet，手机用 Tailscale 引擎 + tun fd
- **会话模型**：多设备并发，owner + subscribers，输出扇出广播
- **UI**：Android 用 WebView（AI 生成维护），iOS 用 SwiftUI 原生
- **配对**：Auth Key 文本输入（不支持扫码，异步场景更灵活）
- **认证**：两层（Tailscale 网络层 + 应用层 device token）
