# Claude Phone 端侧语音输入设计

**日期：** 2026-07-19  
**状态：** 已确认
**范围：** Android 与 iOS 手机客户端

## 1. 目标

把手机端现有语音输入改为严格的设备内语音识别：手机只把识别完成的文字填入消息输入框，用户可以检查和编辑，再按发送；录音数据不发送到 Mac、Claude Phone 服务端、Anthropic 或第三方语音服务。

这项改造的产品价值是：保留接近官方客户端的语音交互，同时让 Claude Phone 可继续连接 Claude Code 或第三方文本模型，并减少上传音频、云端转写和错误上下文造成的额外成本。

## 2. 已选方案

采用手机操作系统提供的端侧语音能力，不随应用安装自有模型：

- Android 只使用 `SpeechRecognizer.createOnDeviceSpeechRecognizer()`。
- iOS 26 及以上优先使用 `SpeechAnalyzer` 与 `SpeechTranscriber`。
- iOS 18–25 只在 `SFSpeechRecognizer.supportsOnDeviceRecognition == true` 时使用旧接口，并强制设置 `requiresOnDeviceRecognition = true`。
- 任一平台不具备端侧能力时，明确提示“当前设备或语言不支持离线语音输入”，不静默退回联网识别。

未选择的方案：

- 应用内置 Whisper：安装体积、内存、耗电和模型维护成本更高，暂不作为 V1 默认能力。
- 云端语音识别：增加隐私、网络、服务依赖和持续费用，不符合本次目标。
- 继续沿用当前默认系统识别入口：无法证明识别只在设备内完成。

## 3. 用户流程

1. 用户点击消息框旁的麦克风按钮。
2. 首次使用时，客户端请求麦克风与语音识别权限。
3. 客户端检查当前设备和语言是否具备端侧识别能力。
4. 支持时进入监听状态，并把识别中的文本实时回填到消息输入框。
5. 最终结果停留在输入框内，不自动发送。
6. 用户检查、修改并手动发送；发送后仍走现有文本消息协议。
7. 权限拒绝、模型未安装、设备不支持或识别失败时，在当前页面显示可理解的错误，不调用联网识别。

第二次点击麦克风或页面退出时停止本次识别。新的识别会替换本次语音产生的草稿，但不会自行覆盖用户在开始语音前已经输入的文字；若输入框非空，识别文本追加到现有草稿末尾，并自动补一个空格或换行所需的自然分隔。

## 4. 共享行为契约

语音识别属于“输入法”，不属于一种新的模型消息类型：

- 音频始终只停留在手机的系统音频与语音框架内。
- 手机到 Mac 的连接只传最终由用户发送的 UTF-8 文本。
- 不新增音频上传接口，不把音频写入项目目录，不保存录音文件。
- 不自动发送识别结果，避免误识别直接消耗模型 token。
- 麦克风按钮具有 `空闲 / 监听中 / 处理中 / 不可用或失败` 状态，并为辅助功能提供相应标签。
- 同一时间只允许一个识别会话；重复启动前先取消并销毁旧会话。

现有 WebView Android 客户端继续通过窄桥接调用原生语音控制器。桥接只暴露“开始或停止识别”，原生回传“草稿文本”和“状态或错误”；JavaScript 不接触音频。

iOS SwiftUI 客户端继续由 `SpeechController` 向 `ChatStore.composer` 回填文字，但控制器内部改成可注入的端侧引擎，便于单元测试。

## 5. Android 设计

### 5.1 能力门槛

- Android 12 / API 31 及以上才调用端侧识别工厂。
- 启动前必须调用 `SpeechRecognizer.isOnDeviceRecognitionAvailable(context)`。
- 仅当返回 `true` 才调用 `createOnDeviceSpeechRecognizer(context)`；绝不调用 `createSpeechRecognizer(context)` 作为后备。
- Android 11 及以下保留应用其他功能，但语音按钮会显示不支持说明。

### 5.2 组件

从 `MainActivity` 中抽出 `OnDeviceSpeechController`：

- 管理 `RECORD_AUDIO` 权限后的启动时机。
- 在主线程创建、启动、停止和销毁 `SpeechRecognizer`。
- 设置 `RecognitionListener`，处理 partial results、final results 和错误码。
- 通过回调向 Activity 交付纯文本与统一状态。
- Activity 再通过已有 WebView 桥接更新输入框和页面状态。

识别请求沿用设备当前语言，使用自由口述模型并开启 partial results。生命周期进入 `onDestroy` 时必须调用 `destroy()`；主动停止、异常和重新开始都要清理旧实例。

### 5.3 错误

Android 系统错误码映射为用户可理解的信息，例如：未听清、无麦克风权限、端侧服务忙、语言模型不可用。任何可能代表网络故障的错误都只展示失败，不触发网络识别。

## 6. iOS 设计

### 6.1 iOS 26 及以上

- 使用 `SpeechAnalyzer` 驱动 `SpeechTranscriber`，识别在设备内执行。
- 根据当前 Locale 检查模块与语言支持。
- 通过 `AssetInventory` 检查并按系统机制安装所需语言资产；资产下载只用于安装 Apple 管理的端侧模型，不上传用户音频。
- 用 `AVAudioEngine` 捕获麦克风 PCM buffer，转换为分析器要求的格式后交给 analyzer。
- 消费转写异步结果，持续更新草稿；停止时完成输入流并释放音频 session、tap 和任务。

### 6.2 iOS 18–25 兼容路径

- 创建当前 Locale 的 `SFSpeechRecognizer`。
- 只有 `supportsOnDeviceRecognition == true` 时才开始。
- `SFSpeechAudioBufferRecognitionRequest.requiresOnDeviceRecognition` 必须为 `true`。
- 不支持时直接进入 `.unavailable` 状态，不允许移除强制端侧标志后重试。

### 6.3 可测试边界

`SpeechController` 依赖一个小型 `OnDeviceSpeechEngine` 协议。生产环境由系统框架适配器实现；测试使用 fake engine 验证权限、状态转换、草稿回填、错误和停止清理，无需在测试中实际访问麦克风。

## 7. 状态与界面反馈

统一状态：

- `idle`：可开始。
- `requestingPermission`：正在请求权限。
- `preparing`：检查能力或准备端侧语言资产。
- `listening`：正在收音和识别，可点击停止。
- `unavailable(message)`：设备、系统版本或语言不支持。
- `denied`：权限被拒绝，并提示去系统设置开启。
- `failed(message)`：本次识别失败，可重试。

Android WebView 页面显示短状态文案并同步麦克风按钮的状态；iOS 在输入区附近显示同样语义的提示。错误不能只写日志，也不能让按钮无响应。

## 8. 测试计划与验收标准

### 8.1 自动化测试

- Android 单元测试：能力门槛、API 版本门槛、错误映射、partial/final 回填、重复启动与销毁。
- Android WebView 契约测试：麦克风桥接、状态回调、草稿追加、结果不自动提交。
- iOS 单元测试：权限拒绝、不支持设备、端侧强制配置、状态机、partial/final 回填、停止清理。
- 仓库回归：现有 Go、Web、Mac、Android AAR 与 iOS 结构检查全部通过。

### 8.2 设备或模拟器验收

- 支持端侧中文识别的 Android 12+ 设备：断网后仍能完成“你好”等短句识别并只填入输入框。
- 不支持端侧识别的 Android 环境：断网或无模型时明确提示，不拉起外部云识别界面。
- iOS 26+ 支持设备：断网识别、实时回填、停止和再次启动均正常。
- iOS 18–25 支持设备：验证请求强制端侧；不支持的语言明确不可用。
- 两个平台：拒绝权限、切到后台、连续点击、空白语音、识别失败和页面退出均不崩溃、不残留麦克风占用。
- 两个平台：语音结果可编辑，只有用户主动发送后 Mac 才收到消息。

无法在本机构造的真实手机硬件场景必须由可注入 fake、平台单元测试和断网能力检查覆盖；交付报告会明确区分“已自动验证”和“需要具体硬件认证”的项目，不把未运行的手工场景宣称为已通过。

## 9. 不在本次范围

- 应用自带 Whisper 或其他本地模型。
- 任何云端语音识别后备。
- 唤醒词、持续监听、后台录音。
- 语音结果自动发送。
- 项目词表同步、自定义热词和标点风格设置。
- 把音频传给 Mac 或模型供应商。

## 10. 文档与可观测性

README 和客户端说明必须明确“端侧语音只传文字”“不支持时不会退回云端”，并列出 Android 12+、具体设备语言资产和 iOS 版本差异。日志只记录状态、平台错误码和耗时，不记录音频，也默认不记录完整转写内容。

## 11. 官方依据

- [Android `SpeechRecognizer` API](https://developer.android.com/reference/android/speech/SpeechRecognizer)：端侧工厂与能力检查从 API 31 提供，并要求销毁实例。
- [Apple `requiresOnDeviceRecognition`](https://developer.apple.com/documentation/speech/sfspeechrecognitionrequest/requiresondevicerecognition)：设为 `true` 可阻止请求通过网络发送音频，但必须先确认设备支持端侧识别。
- [Apple SpeechAnalyzer WWDC25](https://developer.apple.com/videos/play/wwdc2025/277/)：iOS 26 新语音 API 使用设备上的模型资产完成转写。
