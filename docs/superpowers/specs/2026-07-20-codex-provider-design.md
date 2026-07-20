# CodeAfar Codex Provider 设计

**日期：** 2026-07-20  
**状态：** 已确认  
**依赖规格：** `2026-07-19-codeafar-session-provider-design.md`

## 1. 目标

在不改变已确认的新会话体验的前提下，为 CodeAfar 增加可交付的 Codex provider。用户在新建会话 Draft 中选择 `Claude Code` 或 `Codex`；目录、权限、发送、历史、停止和恢复行为保持一致。已有会话继续绑定创建时的 provider，不能跨 provider 恢复上下文。

Codex 复用本机 Codex CLI 的登录态、默认模型、Skills、MCP 和用户配置。CodeAfar 不保存 OpenAI API Key，也不直接调用 OpenAI API。

## 2. 官方接口与技术选择

V1 使用稳定的 `codex exec --json` 非交互接口，不使用仍标为 experimental 的 `codex app-server`，也不引入 TypeScript/Python SDK sidecar。

- 新会话：`codex ... exec --json <prompt>`。
- 后续轮次：`codex ... exec resume --json <thread-id> <prompt>`。
- `thread.started` 提供 Codex thread ID。
- `item.*` 提供 agent message、命令执行、文件修改等事件。
- `turn.completed` 完成本轮；`turn.failed` 和进程异常转换为统一错误。

每轮使用一个短生命周期 Codex 子进程。CodeAfar 的会话队列保证同一会话不会并发发送两轮；子进程退出并释放状态后才向引擎发出 `done`，避免队列在上一进程尚未退出时启动下一轮。

## 3. Provider 能力

注册表同时提供 `claude` 与 `codex`。Codex descriptor 为：

- ID：`codex`
- 名称：`Codex`
- 模型：空值表示使用本机 Codex 配置的默认模型
- 权限：
  - `readOnly`：只读，映射为 `sandbox=read-only`、`approval=never`
  - `workspaceWrite`：工作区访问，映射为 `sandbox=workspace-write`、`approval=never`
  - `fullAccess`：完全访问，映射为 `sandbox=danger-full-access`、`approval=never`

非交互模式不能可靠承载执行中的人工审批，因此 V1 不提供“询问批准”。沙箱之外的动作会失败并返回明确错误。完全访问继续使用现有危险模式二次确认。

Codex 不可用时 descriptor 保留但标记 `available:false` 并提供原因；Claude 仍可正常使用。反过来，本机只有 Codex 时 Mac 应用也必须进入 Ready 状态。

## 4. 进程与会话身份

新增 `CodexProc` 实现现有 `provider.Process`：

- `Start` 初始化进程驱动但不发起模型请求。
- `Send` 为本轮启动 `codex exec` 或 `codex exec resume`。
- `Stop` 终止当前轮次，并阻止该驱动再次发送。
- stdout 按 JSONL 扫描；stderr 仅用于构造非零退出错误，不能泄露认证信息。
- `thread.started` 先更新驱动内的 thread ID，再交给引擎处理。

CodeAfar 自己的 session ID 与 Codex thread ID 分离。会话元数据新增可选 `providerSessionId`：

- 首次收到 `thread.started` 后立即原子持久化。
- 后续发送和应用重启恢复均使用该 ID。
- 旧 Claude 元数据没有此字段时保持兼容。
- Codex 恢复会话缺失该字段时，下一条消息启动新 Codex thread，但保留 CodeAfar 本地历史，并明确记录新的 thread ID。

## 5. 输出转换

引擎根据 provider 选择翻译器，不能继续把所有原始事件当作 Claude stream-json：

- `item.completed / agent_message` → `token`
- `item.started / command_execution` → `tool_use`，工具名为 `Bash`，输入保留 command
- `item.completed / file_change` → 一个或多个文件工具卡片；根据 change kind 映射为 `Write`、`Edit` 或删除文件动作
- `item.completed / mcp_tool_call`、`web_search` 等 → 通用工具卡片，保留可读参数
- `turn.completed` → `done`
- `turn.failed`、顶层 `error`、非零退出且没有终止事件 → `CODEX_ERROR`

同一 item 的 started/completed 不能重复生成工具卡片。未知 item 不得中断流；保留安全的通用回退或忽略纯内部事件。

## 6. Mac 启动与状态

Finder 启动时分别解析 `claude` 与 `codex` 可执行文件并检测版本。应用只要至少一个 provider 可用就启动引擎：

- 状态接口返回两个 provider 的可用信息。
- 新会话 Draft 自动显示可用的引擎选择器。
- 不可用 provider 显示原因且不可选。
- 菜单栏继续显示统一的“引擎运行中”，不把产品状态绑定到 Claude。

命令行新增 `--codex-bin`，保留 `--claude-bin`。现有只配置 Claude 的启动脚本和历史数据无需修改。

## 7. 错误与边界

- 未登录、CLI 版本不兼容、模型请求失败和恢复 ID 无效都转换为用户可理解的 `CODEX_ERROR`。
- 不把 stderr 原样拼接到界面；截断超长错误，并过滤空白。
- 停止会话导致的进程退出不再额外广播错误。
- 权限变更沿用现有“忙时 pending、轮次结束后应用”的语义；空闲时替换 Codex 驱动但恢复同一 thread ID。
- provider thread ID 持久化失败时广播 `ENGINE_ERROR`，不能假装已经具备可恢复性。

## 8. 测试与交付

自动化测试覆盖：

- Codex descriptor、权限参数和二进制可用性。
- 新轮次与 resume 命令参数。
- JSONL 文本、命令、文件修改、done、failed 和未知事件翻译。
- thread ID 捕获、元数据持久化及应用重启恢复。
- Claude 与 Codex 同时注册、单 provider 缺失、未知 provider 拒绝。
- 队列、停止、权限切换、历史加载和旧元数据兼容。
- Web provider 选择器和 provider 专属权限。

真实验收必须使用本机登录态完成：

1. 新建 Codex 会话并收到精确回复。
2. 触发一次只读命令并看到完整工具卡片。
3. 第二轮通过同一 thread ID 恢复上下文。
4. 重建引擎后再次恢复同一 thread。
5. 打包、签名、安装 `/Applications/CodeAfar.app`，在真实窗口创建 Codex 会话。

交付前运行 `make verify`、`go vet ./...`、Android 单测与 APK 构建、Mac 重开测试和独立代码复审，然后提交并推送。

## 9. 本次不做

- Codex app-server、远程 app-server 或云任务接入。
- CodeAfar 内保存 OpenAI API Key。
- 执行中的人工审批 UI。
- 模型目录与模型切换 UI；空模型始终继承本机 Codex 默认值。
- 把 Claude 历史转换或伪装成 Codex thread。
