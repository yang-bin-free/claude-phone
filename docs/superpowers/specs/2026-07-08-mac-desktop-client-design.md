# Mac 桌面客户端设计

- 日期：2026-07-08
- 状态：待评审
- 作者：阿彬 + 小罗
- 范围：Mac 端从"无头 daemon"升级为"带 GUI 的菜单栏常驻桌面客户端"的增量设计。不重写整个项目方案；对现有 README 主文档的同步更新作为实现 plan 的收尾步骤。

---

## 1. 背景与动机

现有 README 方案里，Mac 端只是一个无界面的后台守护进程 `claude-phone-agent`：终端启动、终端看状态、改 YAML 配置。所有"看得见、能点"的操作都在手机端（Android WebView / iOS SwiftUI）。

问题：Mac 本身是开发主力机，用户希望在 Mac 上也能像手机一样直接跟 Claude 干活，并且拥有一个类似 Codex 桌面客户端的完整图形界面。同时 Mac 作为宿主机，还应当承担手机端不具备的管理职责。

本设计把 Mac 端升级为一个**带 GUI 的桌面应用**，同时满足：完整聊天体验、后台常驻（手机随时可连）、Mac 独有的管理控制台。

## 2. 核心定位：三重角色合体

Mac 桌面客户端是三种角色在一个二进制里的合体：

1. **服务端 / 宿主** — 内置引擎（tsnet + session manager + claude 子进程管理），为所有手机提供服务。
2. **本地客户端** — 自带完整聊天 GUI，Mac 上直接跟 Claude 干活，功能与手机端对齐。
3. **控制中心** — 提供手机端不具备的管理功能（设备、工作目录、命令模板、全局会话总览、诊断）。

## 3. 关键设计决策（brainstorming 已定）

| # | 决策 | 选择 | 理由 |
|---|---|---|---|
| D1 | 客户端与引擎的关系 | **方案 A + 后台常驻**：GUI 内置引擎，单进程 | 用户心智负担小（像 Codex 桌面版），同时通过菜单栏常驻达到"手机随时可连"的效果 |
| D2 | 进程/生命周期模型 | 单进程；**关窗口 ≠ 退出**，引擎随菜单栏常驻 | 类似 Docker Desktop / Tailscale Mac 客户端。窗口是引擎的一个前端，可关；引擎在后台持续服务手机 |
| D3 | UI 技术栈 | **全 Go：WebView（wails 或 webview_go）+ systray 菜单栏** | 保持项目"全 Go、单二进制、零运行时依赖"的核心哲学；聊天 UI 与 Android 端复用同一份 HTML/JS |
| D4 | 聊天 UI 复用策略 | **分层复用**：逻辑层 100% 共享、结构层大部分共享、样式层分平台 | 值钱且易出 bug 的是逻辑（流式渲染、断线重连、权限时序），复用它是核心收益；样式差异用独立 CSS 解决 |
| D5 | Mac 本地聊天怎么连引擎 | **走 localhost WebSocket，与手机同协议** | 协议 100% 统一，Mac 会话自动进"全局会话总览"，可被手机接管/共享；localhost 开销可忽略 |
| D6 | 界面结构 | **主窗口双区（聊天 / 管理）+ 菜单栏常驻** | 日常聊天与后台管理职责分离；菜单栏放最高频操作，契合后台常驻习惯 |
| D7 | 无头 CLI（原 mac-agent）去留 | **保留为无头模式**，与 GUI 共享 `pkg/engine` | 抽出 `pkg/engine` 后成本几乎为零；服务器/无 GUI 场景仍可用 |
| D8 | 管理协议 | **独立 `adminproto`，手机无 admin 权限** | 管理消息是本地 GUI 专属，与聊天协议关注点不同；独立协议守住"手机永远调不到管理接口"的安全边界 |

## 4. 整体架构

```
┌──────────────────── Claude Phone.app (单一 Go 二进制) ────────────────────┐
│                                                                          │
│  ┌─ GUI 层 (WebView + systray) ─────────────────────────┐                │
│  │  菜单栏图标 🟢  ·  主窗口(可关，引擎不停)              │                │
│  │  ┌─────────────┬──────────────────────────────────┐  │                │
│  │  │ 聊天区       │  管理区                            │  │                │
│  │  │ (复用手机UI) │  (Mac 独有: 设备/目录/模板/总览/诊断)│  │                │
│  │  └─────────────┴──────────────────────────────────┘  │                │
│  └────────────────────────┬─────────────────────────────┘                │
│                           │ localhost WebSocket (与手机同协议)             │
│  ┌────────────────────────▼─────────────────────────────┐                │
│  │  引擎 (原 claude-phone-agent 的全部能力)               │                │
│  │  tsnet :9876  ·  Session Manager  ·  claude 子进程     │◄──── 手机(远程)  │
│  │  配置热加载  ·  caffeinate  ·  device token 鉴权       │      WS over    │
│  └──────────────────────────────────────────────────────┘      Tailscale  │
└──────────────────────────────────────────────────────────────────────────┘
```

设计原则：**GUI 是引擎的又一个客户端，引擎完全不感知 GUI 的存在。** 引擎层就是原 `claude-phone-agent` 的能力，一行不用为 GUI 改；新增的全是 GUI 层和管理 API。

## 5. 组件划分与代码结构

```
claude-phone/
├── cmd/
│   ├── mac-app/main.go          ★新增: Mac 桌面 App 入口 (systray + webview + 拉起引擎)
│   ├── mac-agent/main.go        ⚠️ 保留: 瘦身为"只启引擎不启 GUI"的无头入口
│   └── android-lib/main.go      (不变)
│
├── pkg/
│   ├── protocol/messages.go     (复用: 三端共享聊天协议 + 错误码，零改动)
│   ├── session/manager.go       (复用: 会话管理，引擎核心)
│   ├── engine/                  ★重构: 把原 mac-agent 引擎能力抽成可复用包
│   │   ├── engine.go            #  tsnet + WS server + session + claude 子进程 + caffeinate + 配置热加载
│   │   └── admin.go             ★新增: 管理能力 API (设备/目录/模板/会话总览/诊断)
│   └── adminproto/messages.go   ★新增: 管理专用协议 (仅本地 GUI ↔ 引擎)
│
└── web/
    ├── chat/                    ★拆分: 聊天 UI (Mac + Android 共享逻辑与结构)
    │   ├── chat.js              #  逻辑层 100% 复用
    │   ├── index.html           #  结构层复用
    │   ├── core.css             #  基础样式共享
    │   ├── mobile.css           #  手机专属 (大触摸区/单列/底部输入)
    │   └── desktop.css          ★新增: Mac 专属 (双栏/侧边栏/悬停/快捷键)
    └── admin/                   ★新增: 管理区 UI (仅 Mac 加载)
        ├── admin.html           #  设备/目录/模板/会话总览/诊断
        ├── admin.js             #  走 adminproto
        └── admin.css
```

### 组件边界

| 组件 | 职责 | 依赖 | 谁能访问 |
|---|---|---|---|
| `pkg/engine` | 引擎核心（网络/会话/进程/配置/防睡眠），无 UI 感知 | tsnet, session, protocol | GUI(本地) + 手机(远程) |
| `pkg/engine/admin.go` | 管理操作（列设备、生成 key、改配置、终止任意会话、诊断） | engine 内部状态 | **仅本地 GUI**（adminproto） |
| `pkg/adminproto` | 管理协议消息定义 | — | 本地 GUI ↔ 引擎 |
| `web/chat` | 聊天前端：逻辑/结构共享，样式分平台 | 聊天协议 | Mac + Android 共用一份 |
| `web/admin` | 管理前端 | adminproto | 仅 Mac |
| `cmd/mac-app` | 桌面 App 入口：systray + webview，进程内拉起引擎 | engine, web 资源 | — |
| `cmd/mac-agent` | 无头入口：只启引擎 | engine | — |

### 聊天 UI 分层复用（D4 细化）

| 层 | 内容 | 手机 vs Mac | 复用 |
|---|---|---|---|
| 逻辑层 | WS 连接、协议收发、流式 token 拼接、会话状态机、工具调用解析、消息队列 | 完全一样 | 100% 复用（`chat.js`） |
| 结构层 | DOM：消息气泡、输入框、工具调用折叠块、权限卡片 | 基本一样 | 大部分复用（`index.html`） |
| 样式层 | 布局、间距、字号、触摸区、悬停、窗口 vs 全屏 | 明显不同 | 分开写（`core.css` + `mobile.css` / `desktop.css`） |

Mac 加载 `core.css + desktop.css`（双栏：左会话列表常驻 + 右聊天，鼠标悬停操作，`⌘K` 切会话、`⌘Enter` 发送，窗口可调）；Android 加载 `core.css + mobile.css`（单列、大触摸区、底部输入、语音）。

## 6. Mac 独有管理功能

| 分类 | 功能 |
|---|---|
| ① 引擎/服务 | 启停引擎、开机自启开关、监听端口与 tsnet 状态、`claude --version` 兼容性检查结果、**并发会话上限 `maxConcurrentSessions` 配置（对应 README §4.5，改 `config.yaml`）** |
| ② 设备管理 | 生成一次性 Auth Key（GUI 点按 + 二维码，手机扫码配对）、已连设备列表（在线状态/device token/最后活跃）、踢设备下线 / 吊销 token |
| ③ 工作目录 | 管理 `projects.yaml`（增删可访问目录，手机新建会话时从此选）、目录级默认权限 |
| ④ 命令模板 | 管理 `templates.yaml`（预设命令，手机端快捷发送） |
| ⑤ 全局会话总览 | 跨设备视角看所有会话（创建设备/subscribers/共享状态）、强制终止任意会话、**查看 `dormant`（Agent 崩溃后待恢复）会话并一键 `--resume`（对应 README §4.14）** |
| ⑥ 权限规则 | **查看 / 删除已记忆的"始终允许"规则 `permission-rules.json`（对应 README §4.9；手机端设置也有，Mac 管理区为完整入口）** |
| ⑦ 系统/诊断 | caffeinate 状态、资源占用（CPU/内存/活跃进程数）、日志查看、消息历史存储位置 |

> 以上管理项与 README 主方案对应章节保持一致：新增的 §4.5 并发上限、§4.9 权限规则持久化、§4.14 崩溃恢复均在 Mac 管理区暴露对应的 GUI 入口。

## 7. 数据流

### 数据流 1：Mac 本地聊天（走 localhost，与手机同协议）

```
Mac 用户在聊天区输入
  → WebView chat.js 发 {"type":"text",...}
  → localhost WebSocket (127.0.0.1:9876)
  → 引擎 Session Manager (与手机消息合并到同一 session)
  → translate → claude 子进程 stdin
  → claude stdout (stream-json)
  → 扇出广播给该 session 所有 subscribers
  → 本地 WebView + 所有订阅的手机 同时收到流式输出
```

收益：Mac 起的会话手机能看到并加入；手机起的会话 Mac 也能接管。真正的三端一致。

### 数据流 2：Mac 管理操作（走 adminproto，仅本地）

```
Mac 用户在管理区点"生成配对码"
  → admin.js 发 adminproto {"action":"gen_auth_key"}
  → localhost WebSocket (带本地 admin 凭证)
  → 引擎 admin.go 校验来源=本地 → 调 tsnet 生成一次性 Auth Key
  → 返回 key + 二维码数据 → WebView 渲染二维码
  → 手机扫码配对
```

## 8. 安全边界

| 通道 | 来源 | 鉴权 | 能力 |
|---|---|---|---|
| 聊天协议 | 本地 GUI + 远程手机 | device token | 聊天、会话操作（受 owner 规则约束） |
| 管理协议 `adminproto` | **仅本地 GUI** | 本地 admin 凭证（非 device token） | 设备管理、改配置、终止任意会话、生成 key |

如何保证"仅本地"：引擎区分两类连接。

- **本地 GUI 连接**：走 `127.0.0.1` 环回口（不经 tsnet），启动时进程内传一个只有 GUI 知道的 admin 凭证 → 才开放 adminproto。
- **远程手机连接**：走 tsnet 虚拟网口，只有 device token → 只能访问聊天协议，adminproto 消息直接拒绝（返回 `PERMISSION_DENIED`）。

即便某台手机 token 泄露，攻击者也碰不到管理接口——admin 通道只认环回口来源 + 进程内凭证。

## 9. 错误处理

| 场景 | 行为 |
|---|---|
| 引擎启动失败（端口占用 / claude 版本不兼容） | 菜单栏图标变红 + 主窗口顶部横幅报错，GUI 仍可打开看诊断 |
| 关主窗口 | 引擎继续，菜单栏常驻，手机不受影响 |
| GUI ↔ 本地引擎 WS 断开（罕见） | GUI 自动重连 localhost，期间显示"重连中" |
| 退出 App（菜单栏 Quit） | 优雅关闭：通知在线手机 → 停 claude 子进程 → 释放 caffeinate → 退 tsnet |
| 手机发来 adminproto 消息 | 拒绝 + 记安全日志 |

## 10. 菜单栏（systray）行为

```
🟢 Claude Phone            ← 图标颜色即状态
─────────────────────────
引擎运行中 · 端口 9876
在线设备: 2 台              ← 实时
活跃会话: 1 个
─────────────────────────
📱 生成配对码…             ← 高频操作，弹二维码
🖥  打开主窗口              ← 唤起/前置主窗口
─────────────────────────
⏸  暂停引擎 / ▶ 启动引擎
⚙️  开机自启   ☑           ← 勾选项
─────────────────────────
退出 (优雅关闭)
```

- **图标状态**：🟢 引擎正常有连接 / ⚪️ 引擎运行无设备 / 🔴 引擎故障（端口占用、claude 不兼容）。
- **首次启动**：App 首次打开 → 引擎自检（claude 版本、tsnet 状态）→ 若 tsnet 未配对，引导页提示"这是宿主机，无需配对；点生成配对码给手机用"→ 主窗口默认停在管理区"设备"页。

## 11. 测试策略

遵循 CLAUDE.md 铁律：逻辑层尽量自动化，UI 层必须真实浏览器/WebView 交互，禁止 curl 直调 API 冒充测试。

| 层 | 测什么 | 怎么测 |
|---|---|---|
| 引擎单元测试 | session 合并、扇出广播、admin 来源鉴权、adminproto 拒绝远程 | Go table-driven test，纯逻辑无 UI |
| 本地/远程双通道集成 | GUI 走环回口能调 admin；模拟远程连接调 admin 被拒 | Go 起引擎 + 两个 WS 客户端（一环回、一模拟远程），断言权限 |
| GUI 端到端 | 菜单栏启停、生成配对码弹二维码、聊天区发消息看流式、管理区增删目录后引擎生效 | 真实驱动 WebView（真实点击/输入，非 curl）+ 截图留证 |

关键断言示例：
- 聊天区发消息 → 引擎 session 出现该消息 → 有"手机"订阅者也收到（验证扇出）。
- 管理区加工作目录 → 引擎 `projects.yaml` 实际写入 → 新建会话时目录列表出现它（navigate 回去确认持久化）。
- 模拟远程 token 连接后发 `gen_auth_key` → 收到 `PERMISSION_DENIED` + 安全日志有记录。

## 12. README 主文档同步（实现 plan 收尾步骤）

设计落地后回改 README：
- `2.2 三端差异`表：Mac 的 UI 层从"—"改为"WebView"，构建产出/分发相应更新。
- `4. Mac 端设计`：daemon → 带 GUI 的常驻 App，补充双区界面与管理功能。
- `3. 项目结构`：新增 `cmd/mac-app`、`pkg/engine`、`pkg/adminproto`、`web/admin`，`web/chat` 拆分说明。
- `7. 实现阶段`：插入 Mac App GUI 相关 phase（引擎抽包 → adminproto → mac-app 壳 → 管理区 UI → 聊天区 desktop 样式 → 端到端）。

## 13. 非目标（YAGNI）

- 不在本设计内重写整个 README 主方案（仅列同步点）。
- 不涉及 iOS 端改动（iOS 仍按 V2 SwiftUI 规划）。
- 不引入 Swift/Rust/Node 工具链（坚持全 Go + WebView）。
- 不做多 Mac 互控（README 已标记为 V2）。
