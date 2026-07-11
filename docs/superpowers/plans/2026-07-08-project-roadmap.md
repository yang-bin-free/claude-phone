# Claude Phone 整体任务规划与并行协调

- 日期：2026-07-08
- 作者：阿彬 + 小罗
- 用途：两个会话（两条开发线）同时推进时的**共同参照**，划清任务边界、依赖、共享交界与防撞车策略。

---

## 1. 当前状态快照

- **已完成**：P0a；P0b Android Tailscale AAR/VpnService 桥；P1/P2 引擎核心与可靠性；Mac 桌面本地管理 API、共享聊天 UI、原生 WebView；Android WebView/语音/配对；干净克隆构建与发布包校验。
- **进行中（线 A）**：P0c 实机跨网络验收，等待 Android 设备与有效 Tailscale Auth Key。
- **已实现待验证（线 B）**：Mac 桌面 GUI 的真实窗口/菜单栏行为；Android 真机 VPN；iOS 18 Xcode 签名构建、Packet Tunnel 与跨 tailnet 联调。

## 2. 两条并行线

整个项目在 P3 之前可拆成两条**基本独立**的开发线：

```
        ┌─────────── 线 A: 手机接入线 (另一会话) ───────────┐
P0a ✅ ──┤ P0b Android Tailscale 引擎 (进行中)              │
        │   → P0c 跨网络连接验证                            │
        └───────────────────────┬────────────────────────┘
                                │
        ┌─────────── 线 B: Mac 引擎/GUI 线 (本会话) ────────┐
        │ Plan 1 引擎核心 → Plan 2 Mac 桌面 GUI            │
        └───────────────────────┬────────────────────────┘
                                ▼  两线汇合
                     P3 Android 完整 → P4 打磨 → P5 开源 → P6 iOS
```

| | 线 A（手机接入·另一会话） | 线 B（Mac 引擎/GUI·本会话） |
|---|---|---|
| 目标 | 手机如何加入 Tailscale 网络并连上 Mac | Mac 引擎核心 + Mac 桌面客户端 |
| 主要目录 | `pkg/androidlib/`、`android/` | `pkg/protocol`、`pkg/session`、`pkg/engine`、`cmd/mac-agent`、`cmd/mac-app`、`web/` |
| 对应 README Phase | P0b → P0c → P3(Android 侧) | P1 → P2 → Plan2(Mac GUI) |
| 参照文档 | README §2.4 / §6.1 | `specs/2026-07-08-mac-desktop-client-design.md`、`plans/2026-07-08-engine-core.md` |

## 3. 共享交界（唯一需要协调的耦合点）

**`pkg/protocol`（三端共享 JSON 协议）** 是两条线唯一的强耦合点：Mac 引擎、Android 端、iOS 端都要用同一份消息定义与错误码。

**协调决策：**
1. **`pkg/protocol` 由线 B 在 Plan 1 Task 2 先落地并提交**（协议是纯数据结构，最稳定）。线 A 需要协议时直接 import，不各写一份。
2. 协议若需扩字段（如线 A 联调发现缺消息），**改动集中到 `pkg/protocol` 单个 PR**，两边评审后合入，不在各自分支私自加。

## 4. 防撞车策略（重要）

**现状风险**：两个会话都在**同一工作目录 + 同一 `master` 分支**上直接改，线 A 已有 830 行未提交。同时写极易互相覆盖 / 合并冲突。

**建议（按推荐度排序）：**

- **[强烈推荐] 各开 git worktree + 独立分支**
  - 线 A：`feat/android-tailscale`（另一会话）
  - 线 B：`feat/engine-core`（本会话）
  - 各自 worktree 物理隔离目录，互不干扰；只在 `pkg/protocol` 定稿、以及最终汇合时通过 PR 合并。
- **[最低要求] 若坚持同目录**：线 A 先把 `pkg/androidlib/tailscale/` **提交掉**（别一直挂未提交），线 B 只碰 `pkg/protocol`/`pkg/session`/`pkg/engine`/`cmd`/`web`，两边严格不碰对方目录。

> 边界铁律：**线 A 只写 `pkg/androidlib/` 与 `android/`；线 B 不碰这两个目录。`pkg/protocol` 归线 B 定稿。** 越界前先在文档/沟通里对齐。

## 5. 里程碑与汇合点

| 里程碑 | 线 A 交付 | 线 B 交付 | 汇合验证（pass/fail 标准） |
|---|---|---|---|
| **M1 基础** | P0b：tun fd 打通，VpnService 拿到隧道 | Plan 1：引擎核心测试全绿，mac-agent 可起监听 | 各自单元 / 集成测试通过 |
| **M2 网络握手** 🎯 | Android 引擎能拨号 | Mac tsnet 能监听（`TsnetDir` 生效） | **P0c：Android 引擎 ↔ Mac tsnet 跨网络 TCP 连接成功**（项目最高风险点） |
| **M3 客户端** | P3-a：Android WebView Chat UI + 语音 + 配对 | Plan 2：Mac 桌面 GUI（systray + 聊天区 + 管理区） | 各端能连本地/远程引擎并渲染流式输出 |
| **M4 联调** 🎯 | 手机连真实 Mac 引擎 | Mac App 连自身引擎 | **手机 + Mac 同时订阅同一 session，双端都看到 claude 流式输出**（三端一致性验证） |
| **M5 发布** | 合并收尾 | 合并收尾 | P4 打磨 → P5 开源（README/Release） |

## 6. web/chat 复用协调（M3 阶段）

Mac（线 B）与 Android（线 A）共享 `web/chat` 的**逻辑层 + 结构层**，样式分平台（见 spec §5 D4）：

- `chat.js` / `index.html` / `core.css`：**两线共用**，改动需双方知情。
- `mobile.css`：线 A 维护（Android）。
- `desktop.css`：线 B 维护（Mac）。

建议：`web/chat` 首次骨架由**先到 M3 的那条线**搭出 `chat.js + index.html + core.css`，另一线在其上加自己的平台 CSS。谁先搭在群里同步一声。

## 7. 本会话（线 B）的下一步

1. Android 设备与 Auth Key 就绪后执行 **M2（P0c）** 实机联调。
2. 在非沙箱 macOS 桌面验证菜单栏、关窗隐藏和优雅退出。
3. 配置发布者签名凭据后完成 Mac 公证和 Android release 签名。

## 8. 未纳入当前规划（V2+，README §8）

Git Diff 查看、cron 定时任务、多 Mac 支持、@file 引用、暗黑模式、审计日志、文件浏览器等——均为 V2/V3，不在本轮并行规划内。
