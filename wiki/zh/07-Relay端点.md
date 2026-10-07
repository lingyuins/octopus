# Relay 端点、代理池、号池与模型映射

### 🌐 公共 Relay 端点

公共 relay API 同时支持 OpenAI 风格和 Anthropic 风格客户端：

- OpenAI 风格客户端：`Authorization: Bearer sk-octopus-...`
- Anthropic 风格客户端：`x-api-key: sk-octopus-...`

| 类别 | 路径 | 说明 |
|------|------|------|
| OpenAI 兼容 LLM | `/v1/chat/completions`、`/v1/responses`、`/v1/embeddings`、`/v1/models` | JSON 请求 / 响应 |
| Anthropic 兼容 LLM | `/v1/messages` | Anthropic 风格请求 / 响应 |
| JSON 媒体 / 工具类 | `/v1/images/generations`、`/v1/audio/speech`、`/v1/videos/generations`、`/v1/music/generations`、`/v1/search`、`/v1/rerank`、`/v1/moderations` | 复用同一套分组 / 重试 / 熔断逻辑 |
| Multipart 媒体类 | `/v1/images/edits`、`/v1/images/variations`、`/v1/audio/transcriptions` | 透传 multipart 上传 |

当上游支持 `stream=true` 时，JSON 媒体类端点也可以直接透传 SSE 流。

语义缓存当前会评估非流式和流式的 OpenAI Chat 与 OpenAI Responses 文本请求（流式缓存命中会从 SSE 会话缓冲区重放）。Anthropic、embeddings 以及媒体 / 工具类端点都会直接旁路缓存，继续走正常 relay 链路。

**日志模块：双视图**

日志模块提供两个可切换视图：

- **转发日志** —— 每次转发请求一条记录，含候选、重试和费用明细（见下文）
- **错误日志** —— 收集后端 panic 和错误（附请求方法 / 路径、客户端 IP、User Agent）以及前端异常（未捕获错误 / 未处理的 Promise 拒绝，附页面 URL 和堆栈）。条目存主库，保留条数有内部上限，超限后周期清理最旧一半；视图内还支持一键清空

**转发日志详情：**

日志模块详情采用双面板：左侧可切换「分组 / 请求体」，右侧展示响应 JSON，底部显示用量、耗时和费用。候选行展示模型图标、渠道/密钥名称，以及日志记录的成功、失败、跳过或熔断状态；展开候选行可查看每次重试和原因。仅当日志明确记录冷却秒数时展示冷却标签，它不是实时倒计时。当前同名分组的候选可补充列表，标为「未记录」；已删除、改名或经过模型映射的分组可能只显示日志中记录的候选。窄屏上下排列两个面板。

**Zen 直连模型路由：**

以 `zen/<model>` 前缀发起的请求会绕过分组模型映射，直接路由到上游模型。Octopus 会根据模型名进行智能渠道类型检测（如 Claude → Anthropic，Gemini → Gemini，GPT → OpenAI）。

**Response ID 亲和性：**

对于 OpenAI Responses API，引用同一 response ID 的后续请求会自动路由到同一上游渠道，以保持对话连续性。

**模型映射：**

全局模型名改写规则在 relay 管线中于分组解析之前生效。规则支持精确、通配符（glob）和正则匹配，带优先级排序和可选分组作用域。

---

### 🌍 代理池

可从应用外壳工具栏访问的共享代理配置池：

- 命名代理配置，支持 URL、协议（SOCKS5 / HTTP / HTTPS）、启用/禁用和备注
- 4 种代理模式：`direct`、`system`、`pool`、`inherit`
- 针对可配置测试 URL 的代理连通性测试
- **引用树**：展示哪些站点、站点账号、托管渠道和渠道使用了每个代理
- 引用跳转导航，可深链到引用实体
- 当代理有活跃引用时，禁止删除

---

### 👥 号池

管理可被 relay 调度使用的上游账号（Anthropic、OpenAI、Gemini、Grok、Volcengine、自定义）的一级模块：

- 号池列表支持关键字搜索；新建/编辑弹窗可配置名称、描述、调度策略、默认并发、冷却时间与启用状态；删除号池需二次确认
- 号池详情页展示摘要统计，并支持按关键字、平台、状态筛选账号——桌面端渲染为表格，移动端渲染为卡片
- 账号支持各平台凭据类型（OAuth / API Key / Cookie / upstream / setup token）；受支持的平台可通过 OAuth 登录流程直接授权账号
- 选中账号支持批量刷新 Token、清除错误和连通性测试；单个账号可临时暂停调度
- 账号凭据可批量导入或导出为文件——导出弹窗会警告导出内容包含明文凭据

---

### 🔁 模型映射

全局模型名改写规则，在 relay 管线中于分组解析之前生效：

- **匹配类型**：精确、通配符（glob）和正则
- **目标模型**：改写后的模型名
- **优先级排序**：按优先级顺序评估规则
- **分组作用域**：可选仅应用于特定分组
- **启用/禁用开关**：每条规则可独立开关

---

| [← 首页](../Home_zh.md) | [← 上一页](06-模型广场.md) | [下一页 →](08-分析.md) |