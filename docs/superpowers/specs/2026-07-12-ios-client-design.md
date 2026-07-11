# Claude Phone iOS 18 客户端设计

- 日期：2026-07-12
- 状态：设计自审通过，等待用户书面复核
- 范围：iOS 18、原生 SwiftUI、单 App 内嵌 Packet Tunnel Extension

## 1. 目标与交付边界

本轮把 iOS 纳入 Claude Phone 当前交付范围。交付物包括完整 Swift 源码、Xcode 工程、
Packet Tunnel Extension、Go XCFramework 构建入口、测试与 CI 结构检查。工程应能在安装
完整 Xcode、iOS 18 SDK，并配置 Apple Developer Team 与 Network Extension entitlement
的机器上直接构建。

当前机器只有 Command Line Tools，没有 iPhoneOS SDK 和签名能力，因此本轮在本机完成
Go 测试、脚本语法检查、工程结构校验、plist/entitlement 校验和 Swift 静态规则检查。
IPA、真机 VPN、跨 tailnet 与 TestFlight 验收在苹果工具链和签名条件就绪后执行。

## 2. 选定方案

采用主 App + Packet Tunnel Extension + Go XCFramework：

- `ClaudePhone` 主 App 使用原生 SwiftUI，负责配对、设置、聊天、会话、模板和语音。
- `ClaudePhoneTunnel` 继承 `NEPacketTunnelProvider`，负责系统 VPN 生命周期。
- `ClaudeCore.xcframework` 由 gomobile 从共享 Go 核心生成，承载 Tailscale 用户态网络。
- 主 App 与 Extension 通过 App Group 共享最小配置与状态。
- 隧道建立后，主 App 使用原生 `URLSessionWebSocketTask` 连接 Mac `/ws`。

不采用依赖官方 Tailscale App 的双 App 方案，因为它违反“手机只安装一个 App”的产品
目标；不采用纯 Swift 重写 Tailscale，因为成本、安全风险和维护面均不合理。

## 3. 组件边界

### 3.1 主 App

主 App 包含以下独立单元：

- `PairingStore`：配对表单、VPN 权限请求和首次连接状态机。
- `TunnelController`：保存 `NETunnelProviderManager` 配置，启动、停止和观察隧道。
- `WebSocketClient`：认证、收发 JSON、心跳、消息大小限制和指数退避。
- `SessionStore`：会话列表、选择、恢复、停止和显式 FIFO 状态。
- `ChatStore`：历史加载、token 合并、工具调用、错误和健康状态。
- `SpeechController`：`SFSpeechRecognizer` 权限、识别和文本回填。
- SwiftUI Views：配对、会话列表、聊天、设置和诊断。

各 Store 通过构造参数依赖协议接口，测试不依赖真实 VPN、WebSocket 或语音服务。

### 3.2 Packet Tunnel Extension

Extension 只负责：

1. 从 App Group 读取一次性启动配置。
2. 配置 `NEPacketTunnelNetworkSettings`。
3. 启动 Go/Tailscale 引擎并桥接 packet flow。
4. 把连接状态和最后错误写回 App Group。
5. 停止时释放 Go 引擎和网络资源。

Extension 不承载聊天协议、UI 或长期凭证管理。

### 3.3 Go XCFramework

iOS 导出层放在独立、可 gomobile bind 的 Go 包中，暴露窄接口：启动、停止、当前状态和
错误文本。现有 Android API 不直接成为 Swift 公共 API，平台差异留在各自 adapter 中，
底层 Tailscale 实现继续共享。

## 4. 配置与安全

App Group 保存：Control URL、Tailscale 状态目录位置、隧道状态和最后错误。Device Token
只供主 App 的 WebSocket 使用，作为应用凭证存入 Keychain，不写入 App Group。

Tailscale Auth Key 只在首次注册时使用。主 App 将其临时写入共享容器，Extension 读取后
立即删除；无论启动成功、失败或超时，都必须执行清理。Tailscale 登录状态由 Go 核心保存
在 App Group 容器内的专用目录，后续启动不再需要 Auth Key。

退出账号会停止隧道、删除 Keychain Device Token、共享配置和 Go/Tailscale 状态目录。

## 5. 数据流

1. 用户在配对页输入 Mac 地址、Device Token、Auth Key 和可选 Control URL。
2. App 校验字段，保存非敏感配置，请求系统 VPN 权限。
3. `TunnelController` 启动 Extension；Extension 启动 Go 引擎并报告状态。
4. 隧道就绪后，App 连接 Mac `/ws`，首包发送 device auth。
5. hello 后并行请求会话、项目和模板。
6. 新建会话时发送选定项目目录与权限模式；未授权目录由 Mac 拒绝。
7. 选择休眠会话时恢复 Claude，并请求最近 500 条历史。
8. 文本增量批量刷新 UI；工具、队列、错误和健康状态使用独立消息模型。
9. WebSocket 断线在 VPN 有效时按 1、2、4、8、15 秒退避重连；VPN 失效时停止重试。

## 6. SwiftUI 交互

iPhone 使用 `NavigationStack`：根页面为会话列表，聊天页为详情。新建会话入口同时选择
项目和严格/审阅/信任权限模式。聊天页面提供模板横向列表、原生消息列表、工具调用卡片、
队列提示、健康状态和文本输入。

语音按钮按需请求 Speech 与麦克风权限，识别结果只回填输入框，由用户确认发送。设置页
展示 VPN、WebSocket、Mac、协议和 Claude 版本状态，并提供停止 VPN、重新配对和清除账号。

## 7. 错误处理

用户可见错误分为：VPN 权限拒绝、Tunnel 启动超时、Tailscale 登录失败、Mac 不可达、
Device Token 无效、协议不兼容、语音不可用。每类错误提供明确的重试或设置入口。

WebSocket 限制单条消息大小，Codable 模型允许未知字段；未知 type 进入诊断日志而不是终止
连接。Tunnel 启动失败必须释放引擎并清除 Auth Key。Extension 崩溃后，主 App 从共享状态
读取最后错误并引导重启。

## 8. 工程与构建

仓库新增：

- `ios/ClaudePhone.xcodeproj`
- `ios/ClaudePhone/` 主 App Swift 源码与资源
- `ios/ClaudePhoneTunnel/` Packet Tunnel Extension
- `ios/Shared/` 协议模型和 App Group 常量
- `ios/ClaudePhoneTests/` 单元测试
- `scripts/build-ios-framework.sh`
- `scripts/validate-ios-project.sh`

Deployment Target 固定为 iOS 18.0。Bundle ID 和 App Group 使用可替换的工程配置变量；
仓库不提交 Team ID、证书或 provisioning profile。Debug 工程可在配置开发团队后签名。

## 9. 验证策略

当前环境执行：

- 全量 Go 测试与 race 测试。
- Go iOS adapter 的宿主平台单元测试。
- XCFramework 构建脚本 `bash -n`。
- Xcode project、targets、plist、entitlements、App Group 和源码引用结构检查。
- Swift 源码禁用硬编码 token、Auth Key 和 Team ID 的安全检查。

具备完整 Xcode 后执行：

- iOS 18 模拟器编译和 XCTest。
- 真机 Packet Tunnel 启停、首次注册和状态恢复。
- iPhone 与 Mac 跨 tailnet WebSocket 连接。
- Mac+iPhone 同会话的历史、流式 token、工具、队列与健康状态一致性。
- 语音权限、识别、后台切换、断网重连和退出账号清理。

## 10. 验收标准

源码阶段通过条件：工程结构检查和所有可运行测试全绿，构建说明列出唯一仍需的 Xcode、
Apple Developer Team、Network Extension entitlement 和真机条件。

最终真机通过条件：单个 iOS App 能建立内嵌 Tailscale 隧道、连接 Mac、创建/恢复会话、
显示原生流式聊天、使用语音输入，并在后台/断网后恢复；Auth Key 不在持久化存储中残留。
