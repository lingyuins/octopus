# Ops Telemetry 总览设计

## 背景

当前 `运维中心` 已经有 `Cache / Quota / Health / System / Audit` 5 个标签页，但它们是分散的运维视图：

- `Health` 只展示数据库、缓存、任务运行态和分组健康卡片。
- `Cache`、`Quota`、`System` 各自覆盖单一主题，缺少统一总览入口。
- 页面里没有一个“先看整体运行态，再下钻到具体页”的汇总层。

用户给出的目标不是只给现有 `Health` 补一个“文件内健康显示程度”，而是让 `运维中心` 具备类似 OmniRoute 的 `Telemetry` 总览能力，把运行态、请求流量、会话、缓存、Provider 健康和现有运维入口汇总到一个新的首屏总览页里。

## 目标

1. 在 `运维中心` 新增一个 `Telemetry` 总览 Tab，作为统一运维入口。
2. 保留现有 `Cache / Quota / Health / System / Audit` 页面，不打散原有功能。
3. 后端新增一个聚合读取接口，把现有可复用的运维数据和新增的轻量运行态信号整合到一份响应里。
4. 首期只做“总览 + 下钻”，不把现有设置页、修复页、编辑页复制进来。

## 非目标

1. 不把 `Ops` 改成单页控制台，不重做整个运维中心导航。
2. 不删除或重写现有 `Cache / Quota / Health / System / Audit` 页面。
3. 不在首期引入新的数据库表。
4. 不在首期补齐 OmniRoute 示例中的所有“主动修复”能力。
5. 不为了凑指标而伪造后端不存在的历史趋势数据。

## 方案概述

在 `Ops` 下新增第一个标签页：`Telemetry`。

页面定位：

- `Telemetry` 只做总览，不承担深度编辑职责。
- 用户进入运维中心后，先在 `Telemetry` 看“系统现在是否健康、是否有流量、是否有异常、该点进哪个子页继续排查”。
- 详细数据、配置和排查动作仍然回到现有标签页。

信息层次按已经确认的 A 方案执行：

1. 顶部核心指标
2. `Runtime Signals`
3. `Database Health`
4. `Session & Quota Activity`
5. `Prompt Cache`
6. `Provider Health`
7. `Drilldown Shortcuts`

## 信息结构

### Ops 顶层结构

调整为：

1. `telemetry`
2. `cache`
3. `quota`
4. `health`
5. `system`
6. `audit`

`Telemetry` 放在第一个标签位，作为新的默认页签。

### Telemetry 页面结构

#### 1. 顶部核心指标

固定展示 6 个高优先级指标：

- `Uptime`
- `Total Requests`
- `Avg Latency`
- `Error Rate`
- `Active Connections`
- `Memory Usage`

用途：

- 让用户一眼判断当前实例是否“活着、在跑、负载如何、有没有明显错误”。

#### 2. Runtime Signals

展示运行态摘要，不做复杂图表，首期采用轻量卡片 + 简化趋势条：

- `p95 Latency`
- `Throughput`
- `RSS / Heap`
- `Latency trend`
- `Throughput trend`
- `Memory trend`

说明：

- 首期的“trend”不是完整折线图，而是轻量摘要条或分段占比条。
- 这样可以先把数据来源打通，不需要为了视觉效果引入额外图表复杂度。

#### 3. Database Health

展示数据库和运维数据完整性的汇总状态：

- `Status`
- `Issues`
- `Repairs`
- `Diagnose` / `Auto-Repair` 动作入口占位

首期设计约束：

- 页面需要预留动作区，但首期允许先只实现“诊断摘要 + 禁用按钮 / 占位按钮”。
- 如果仓库里当前没有完整的自动修复逻辑，按钮不能伪装成已可用功能。

#### 4. Session & Quota Activity

把会话活动和配额活动合在一个区块：

- `Active Sessions`
- `Sticky-bound Sessions`
- `Quota Alerts`
- `Sessions by API Key`
- `Quota Monitors`

用途：

- 统一回答“现在有没有会话”、“sticky 是否在生效”、“配额是否有明显压力”。

#### 5. Prompt Cache

承接现有语义缓存统计，并补一个签名缓存类统计区：

- `Entries`
- `Hit Rate`
- `Hits / Misses`
- `Signature Cache` 摘要
- `Defaults / Tool / Family / Session` 分段占位

说明：

- 仓库当前已有语义缓存运行态统计，这部分应当直接复用。
- “Signature Cache” 在首期如果没有真实分类数据源，则保留为扩展结构，不得伪造具体分类值。

#### 6. Provider Health

从“坏分组列表”扩展为“供给源健康总览”：

- `Providers`
- `Active`
- `Monitored`
- 每个 Provider 的行级状态

每一行优先展示：

- `Provider name`
- `Enabled / disabled`
- `Average latency`
- `Request volume`
- `Success rate`
- `Route/group health hint`

说明：

- 这里的 `Provider` 在 Octopus 里落地为 `Channel` 维度，而不是外部产品的抽象 provider registry。
- 首期按 `Channel` 聚合，不再额外创造新实体。

#### 7. Drilldown Shortcuts

保留对现有页面的下钻入口：

- `Cache`
- `Quota`
- `Health`
- `System`
- `Audit`

用途：

- `Telemetry` 负责指路，不重复造已有复杂页面。

## 后端设计

## 接口新增

新增一个聚合读取接口：

- `GET /api/v1/ops/telemetry`

权限沿用当前 `Ops` 路由：

- `middleware.Auth()`
- `middleware.RequirePermission(auth.PermSettingsRead)`

原因：

- 该页面是当前 `Ops` 的统一总览，权限边界应与现有运维页保持一致。

## 响应结构

新增 `internal/model/ops.go` 中的 Telemetry 结构体族，按以下层次组织：

- `OpsTelemetrySummary`
- `OpsTelemetryHeroMetrics`
- `OpsTelemetryRuntimeSignals`
- `OpsTelemetryDatabaseHealth`
- `OpsTelemetrySessionQuotaActivity`
- `OpsTelemetryPromptCache`
- `OpsTelemetryProviderHealth`
- `OpsTelemetryProviderItem`

返回时保持 `resp.Success(c, data)` 的统一包裹格式。

## 数据来源拆分

### 一类：可直接复用现有 Ops / Analytics 数据

这些字段不应重复造轮子，直接从现有聚合函数拼装：

#### 1. Cache

来源：

- `op.OpsCacheStatusGet()`
- `semantic_cache.Stats()`
- `semantic_cache.GetRuntimeStats()`

可直接支撑：

- `Entries`
- `Hit Rate`
- `Hits / Misses`
- `CurrentEntries / MaxEntries`
- `UsageRate`

#### 2. Quota

来源：

- `op.OpsQuotaSummaryGet()`
- `op.StatsAPIKeyList()`

可直接支撑：

- 活跃有使用的 API Key 数
- exhausted / limited / expired 等计数
- `Sessions & Quota Activity` 中的 quota 摘要

#### 3. Group / route health

来源：

- `op.OpsHealthStatusGet()`
- `op.AnalyticsGroupHealthGet()`

可直接支撑：

- `Database Health` 的基础状态
- `Provider Health` 中的 route/group 健康提示
- 当前 `Health` 页已有的 failing groups 口径

#### 4. System summary

来源：

- `op.OpsSystemSummaryGet()`

可直接支撑：

- 版本、构建时间、数据库类型
- AI Route 服务数
- 当前配置的 channel / group / API key 数

#### 5. 请求总量与平均延迟

来源：

- `op.StatsTotalGet()`
- `op.StatsTodayGet()`
- `op.StatsChannelList()`

可直接支撑：

- `Total Requests`
- `Avg Latency`
- `Error Rate`
- `Throughput`

口径约束：

- `Total Requests = RequestSuccess + RequestFailed`
- `Avg Latency = WaitTime / Total Requests`
- `Error Rate = RequestFailed / Total Requests`

### 二类：首期新增的轻量运行态采集

这些数据当前仓库没有统一对外函数，需要补最小量运行态采集，但不引入新表。

#### 1. Uptime

新增：

- 在后端启动阶段记录进程启动时间。
- `Telemetry` 读取 `time.Since(processStartedAt)`。

不需要写库。

#### 2. Memory Usage / RSS / Heap

新增：

- 在 `op` 层增加进程内内存读取函数。
- 首期优先使用 Go 运行时可直接拿到的内存指标。

口径约束：

- `Heap` 必须来自 Go runtime。
- `Memory Usage` 首期允许使用进程内可稳定获取的总内存摘要，不强依赖额外第三方库。
- 如果无法稳定给出真实 RSS，就把字段命名为更准确的 `Process Memory`，不要冒充 RSS。

#### 3. Active Connections

新增：

- 在 relay 请求入口增加进出计数，维护一个当前 in-flight 请求数。

口径约束：

- 这里统计的是“当前实例内正在处理的公开 relay / media 请求数”。
- 不统计浏览器连接、管理后台请求或 TCP socket 数。

#### 4. Active Sessions

新增：

- 在 `relayStreamSessionStore` 上补只读统计函数。

可统计：

- 当前仍存在于 store 中且未过期的会话数
- 已完成但仍处于 replay TTL 窗口内的会话数

首期 `Active Sessions` 口径定义为：

- 当前未完成的流式会话数

#### 5. Sticky-bound Sessions

新增：

- 在 `balancer/session.go` 上补只读统计函数，遍历 `globalSession`。

可统计：

- TTL 内仍有效的 sticky 映射数量

#### 6. Runtime trend snapshots

新增：

- 在内存中维护最近 N 个窗口摘要，不落库。

首期定义为：

- 固定 12 个点位
- 每 30 秒滚动一次
- 存储：
  - request count delta
  - failed request delta
  - avg latency
  - memory snapshot

原因：

- 首期只需要给 `Telemetry` 自己做轻量趋势摘要，不必改现有统计表结构。

### 三类：首期只预留结构，不强行实现完整能力

#### 1. Database Health 的 Diagnose / Auto-Repair

首期允许：

- 返回 `status / issues / repairs`
- 前端渲染动作区

首期不强制：

- 真正的自动修复执行链

如果没有真实修复逻辑：

- 按钮显示为 `Coming soon` 或 disabled 状态
- 文案明确说明当前仅支持诊断摘要

#### 2. Signature Cache breakdown

当前仓库没有与视觉稿完全对应的“Defaults / Tool / Family / Session”分类缓存统计源。

首期策略：

- 在接口结构中预留该对象
- 没有真实数据时返回 `0` 或 `null`，前端展示 empty/hint
- 不伪造分类统计

## Provider Health 的落地口径

`Provider Health` 首期按 `Channel` 聚合，而不是按更抽象的 provider family 聚合。

每个 `ProviderItem` 包含：

- `channel_id`
- `channel_name`
- `enabled`
- `base_url`
- `request_count`
- `success_rate`
- `average_latency_ms`
- `health_status`
- `health_hint`

来源：

- `request_count / average_latency_ms / success_rate` 来自 `StatsChannelList()`
- `health_status / health_hint` 结合：
  - channel 是否启用
  - group health 中是否有大量失败引用
  - channel key 最近状态码是否异常

排序：

1. 有请求且异常更明显的排前
2. 再按请求量降序
3. 再按名称排序

这样进入 `Telemetry` 时，异常 provider 更容易被先看到。

## Database Health 的落地口径

首期不照搬 OmniRoute 的“stale quota/domain rows and broken combo references”文案，改成适合当前仓库的完整性检查语义。

首期诊断项：

1. 数据库可连通
2. 语义缓存运行态是否与配置一致
3. 后台任务运行态是否正常
4. 分组健康是否存在 `down / degraded`
5. 是否存在孤儿统计项
   - 例如 `StatsAPIKey` 对应的 API Key 已不存在

首期 `Issues` 可按诊断命中数量汇总。

`Repairs` 首期允许固定为 `0`，直到真实修复逻辑落地。

## 前端设计

## 组件结构

新增：

- `web/src/components/modules/ops/Telemetry.tsx`

调整：

- `web/src/components/modules/ops/index.tsx`
- `web/src/api/endpoints/ops.ts`

保留：

- `Cache.tsx`
- `Quota.tsx`
- `Health.tsx`
- `System.tsx`
- `Audit.tsx`

## 前端数据获取

在 `web/src/api/endpoints/ops.ts` 新增：

- `OpsTelemetrySummary` 类型
- `useOpsTelemetrySummary()` hook

查询策略沿用现有 Ops：

- `queryKey: ['ops', 'telemetry']`
- `refetchInterval: 30000`
- `refetchOnMount: 'always'`

如果响应里有可空数组或可空对象：

- 在 `select` 中做默认值归一化
- 避免组件内部到处判空

## 页面布局

### 1. Hero 区

风格与现有 `Ops`、`Analytics` 保持一致：

- 圆角卡片
- 浅色背景
- 不做夸张控制台风格

顶部显示：

- 标题
- 描述
- `last updated`

下方是 `6` 个核心指标卡。

### 2. 双列内容区

采用：

- 左侧：`Runtime Signals`、`Prompt Cache`
- 右侧：`Database Health`、`Session & Quota Activity`

底部整行：

- `Provider Health`
- `Drilldown Shortcuts`

原因：

- Provider 行表天然更宽，适合横向整行呈现。
- `Drilldown Shortcuts` 放在底部，符合“先判断问题，再继续下钻”的阅读顺序。

## 交互设计

### 1. Drilldown

点击快捷入口后，直接切换到对应 `Ops` Tab。

不做新路由跳转。

### 2. Auto-Repair / Diagnose

首期规则：

- 如果后端未提供动作接口，则按钮为 disabled 态
- 文案明确说明当前仅支持查看摘要

### 3. Provider item

首期不需要在 `Telemetry` 里展开复杂明细抽屉。

只需要：

- 行级状态
- 关键信号
- 必要时提示“前往 Health / System / Channel 查看详情”

## 文案与国际化

需要同步更新：

- `web/public/locale/zh_hans.json`
- `web/public/locale/zh_hant.json`
- `web/public/locale/en.json`

新增内容包括：

- `ops.tabs.telemetry`
- `ops.telemetry.*`

要求：

- 三语键保持一致
- 不在组件里写死英文样例词

## 测试与验证

## 后端测试

新增或扩展 `internal/op/ops_test.go`，覆盖：

1. `Telemetry` 聚合结构的组装
2. 请求量、错误率、平均延迟的计算
3. active session / sticky session 统计
4. provider health 排序和状态映射
5. 空数据时的默认响应

如果新增 runtime 采集 helper：

- 单独补对应单元测试

## 前端测试

至少覆盖：

1. `Ops` tab 新增 `telemetry`
2. `useOpsTelemetrySummary()` 的默认值归一化
3. `Telemetry` 页面在空数据下的显示
4. 文案 key 的三语一致性

## 构建验证

执行：

1. `go test ./...`
2. `go build ./...`
3. `cd web && pnpm lint`
4. `cd web && $env:NEXT_PUBLIC_APP_VERSION='dev'; pnpm build`

## 浏览器验收

本地运行后至少确认：

1. `Ops` 默认进入 `Telemetry`
2. 6 个核心指标正常显示
3. `Provider Health` 列表和空态正常
4. `Drilldown Shortcuts` 可以切换到现有标签页
5. 窄屏下不会横向炸裂或导致整页无法滚动

## 分阶段实现建议

### Phase 1

- 新增 `Telemetry` Tab
- 新增 `/api/v1/ops/telemetry`
- 复用已有 cache / quota / health / system 数据
- 新增 uptime / in-flight / active session / sticky session / process memory 采集
- 前端完成总览页

### Phase 2

- 轻量 runtime trend 内存快照
- provider health 更细的健康提示
- database diagnose 摘要细化

### Phase 3

- 真正的 diagnose / repair 动作
- 更完整的缓存分类统计
- 可视化趋势图增强

## 实现边界总结

这次设计的核心是：

- `新增一个 Ops 总览页`
- `后端补一个聚合读取接口`
- `复用现有子页作为下钻目标`

而不是：

- 把现有运维功能推倒重做
- 在首期引入新的持久化模型
- 强行实现没有数据源支撑的 OmniRoute 全量指标
