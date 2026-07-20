# CodeAfar

CodeAfar 是一个从电脑或手机远程使用本机编程 Agent 的客户端。Mac V1 当前支持 Claude Code，协议和会话层已经改造成 provider-neutral；后续可接入 Codex 等第三方 Agent，而不需要重做客户端体验。

它的核心价值是把语音转写、会话状态、目录授权和 UI 留在本地，只把最终确认的提示词交给编程 Agent，减少无效上下文和 token 消耗。

## Mac V1

当前能力：

- 在新会话草稿中选择项目目录、Provider 和权限模式；发送第一条消息时才创建会话。
- 通过原生 Finder 目录选择器授权项目，不接受网页伪造的任意路径。
- Return 发送，Shift+Return 换行；聊天内容可选择和复制。
- Claude 回复流式显示，会话与历史重启后恢复。
- 会话中可调整权限模式；忙碌时延后到当前轮结束，空闲时安全重启并恢复上下文。
- 菜单栏常驻、关窗隐藏、再次打开恢复窗口、暂停/恢复、诊断和开机自启。
- 首次启动把旧 `~/.claude-phone` 数据迁移到 `~/.codeafar`，不覆盖已经存在的新数据。

权限模式：

- `严格`（`default`）：Claude 在执行需要授权的工具操作前询问，适合作为默认值。
- `审阅`（`acceptEdits`）：自动接受文件编辑，其他敏感操作仍按 Claude 的规则处理。
- `信任`（`bypassPermissions`）：跳过权限确认，只应在完全信任的目录中使用。

## 安装并打开 Mac 应用

要求：macOS 12 或更高版本，以及已经可用的 Claude Code CLI。

```bash
make install-mac-app
```

命令会构建、签名校验并原子安装到 `/Applications/CodeAfar.app`，然后启动应用。如果新版本无法启动，会恢复原安装。也可以直接在 Finder 的“应用程序”中双击 CodeAfar。

仅构建发布包：

```bash
VERSION=0.1.0-dev make mac-release
shasum -a 256 -c build/release/SHA256SUMS
```

产物是 `build/CodeAfar.app` 和 `build/release/codeafar-macos-<version>.zip`。仓库构建使用 ad-hoc 签名，公开分发前仍需要 Developer ID 签名和 Apple 公证。

无界面模式：

```bash
make build-agent
./build/codeafar-agent serve
./build/codeafar-agent key --name Pixel
```

## 会话设计

点击“新会话”只创建一个未提交草稿。用户从 Finder 选目录，按需修改 Provider 和权限，然后输入消息；第一次发送会以稳定的 `requestId` 创建会话，收到 `session_created` 后发送带同一消息 ID 的文本，并保留到服务端返回 `text_accepted`。断线后重试会由服务端去重，避免重复会话或重复执行首条消息。

当前 Provider：

| Provider | 状态 | 权限模式 |
|---|---|---|
| Claude Code | 可用 | 严格、审阅、信任、计划 |
| Codex / 第三方 | Provider 接口已就绪，适配器待接入 | 由各 Provider 描述能力 |

## 本地语音输入

语音只负责填写消息草稿，不会自动发送；识别结果追加到原有文字，用户可以继续修改。

- Android 12+：只使用 `createOnDeviceSpeechRecognizer`。设备没有端侧识别服务时明确提示不可用，不回退到网络识别。
- iOS 26+：使用 `SpeechAnalyzer`、`SpeechTranscriber` 和 Apple 管理的本地语言资源。
- iOS 18–25：只在 `SFSpeechRecognizer.supportsOnDeviceRecognition` 可用时启动，并强制 `requiresOnDeviceRecognition = true`。

系统可能需要下载由 Apple/Android 系统管理的语言资源；这是一次性的模型资源准备，不代表语音内容发送到 CodeAfar 或 Agent 服务端。

## 开发与验证

```bash
make verify
make mac-release

cd android
JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home \
ANDROID_HOME="$HOME/Library/Android/sdk" \
./gradlew :app:testDebugUnitTest :app:assembleDebug --no-daemon

cd ..
xcodebuild test -quiet \
  -project ios/ClaudePhone.xcodeproj \
  -scheme ClaudePhone \
  -destination 'platform=iOS Simulator,name=iPhone 17 Pro' \
  CODE_SIGNING_ALLOWED=NO
```

完整 Mac 验收矩阵见 [`docs/testing/mac-v1-acceptance-plan.md`](docs/testing/mac-v1-acceptance-plan.md)。

## 架构

```text
Mac CodeAfar.app
  ├─ Cocoa/WebKit 桌面壳与本地管理页
  ├─ Go WebSocket/HTTP 引擎
  ├─ Provider Registry
  │    └─ Claude Code adapter → 本机 claude CLI
  └─ ~/.codeafar（项目、设备、会话、权限、历史）

Android / iOS
  ├─ 原生 VPN/Tailscale 接入
  ├─ 共享会话协议
  └─ 严格端侧语音 → 可编辑消息草稿
```

主要目录：

- `cmd/mac-app`：Mac 桌面入口。
- `cmd/mac-agent`：Mac 无头 Agent。
- `pkg/engine`：连接、会话、持久化和权限控制。
- `pkg/provider`：Provider 描述与适配器注册表。
- `pkg/desktop`：仅回环开放的桌面服务和原生桥接。
- `web/chat`：Mac/Android 共享聊天界面。
- `android`、`ios`：移动端原生壳与本地语音。

## 安全与隐私

- 桌面管理接口只监听 loopback，并校验进程随机管理 token。
- 手机连接使用 Device Token；跨网络链路由 Tailscale/WireGuard 加密。
- Finder 选择的项目才会加入桌面授权列表。
- token、Auth Key 和语音内容不会写入仓库。
- `信任`模式会放宽 Claude Code 自身的工具确认，CodeAfar 会清晰展示当前模式，但无法替代操作系统沙箱。

## License

MIT。第三方许可证见 `THIRD_PARTY_LICENSES.md`。
