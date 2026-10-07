<div align="center">

<img src="../web/public/logo.svg" alt="Octopus Logo" width="120" height="120">

### Octopus Wiki

**为个人打造的简单、美观、优雅的 LLM API 聚合与负载均衡服务**

[English](Home.md) | [README](../README_zh.md) | [Changelog](../CHANGELOG.md)

</div>

## ✨ 特性

- 🔀 **多渠道聚合** - 支持接入多个 LLM 供应商渠道，统一管理
- 🔑 **多Key支持** - 单渠道支持配置多 Key
- ⚡ **智能优选** - 单渠道多端点，智能选择延迟最小的端点请求
- ⚖️ **负载均衡** - 支持轮询、随机、故障转移、加权分配、智能选择五种策略
- 🤖 **Auto 智能策略** - 先探索样本不足的候选，再优先选择窗口内成功率更高的渠道
- 🧠 **AI 路由、自动分组与条件分组** - 支持在路由页生成整张路由表，在分组编辑弹窗中补全单个分组，并用 JSON 条件控制分组命中
- 🔄 **协议互转** - 支持 OpenAI Chat / OpenAI Responses / OpenAI Embeddings / Anthropic 四种 API 格式互相转换
- 🌐 **多供应商支持** - 内置支持 OpenAI 兼容、Anthropic、Cloudflare、Gemini、Volcengine、MiMo、Codex 以及 Passthrough 透传渠道
- 🛰️ **媒体与工具类中继** - 支持通过同一套分组 / 重试 / 熔断基础设施转发 OpenAI Images、音频、视频、搜索、重排和审核类端点
- 👥 **号池** - 池化管理上游账号（OAuth / API Key / Cookie 凭据），提供专属调度、健康巡检、Token 自动刷新和定时连通性测试
- 📜 **转发日志与错误日志** - 日志双视图：转发日志展示逐候选重试明细，错误日志收集后端 panic 和前端异常
- 🧾 **API Key 治理** - 支持模型白名单、过期时间、费用上限、RPM / TPM 限额、按模型配额，以及 IP / CIDR 白名单
- 🔐 **角色化管理权限** - 内置 `admin`、`editor`、`viewer` 三种角色，并由服务端强制执行权限控制
- 🔑 **WebAuthn / Passkey 登录** — 通过 WebAuthn/Passkey 实现无密码登录和注册，支持可配置 RP 设置
- 🚨 **告警与通知中心** - 统一通知中心聚合系统事件、告警触发和套餐通知，支持 SSE 实时推送
- 📦 **套餐监控** - 跟踪上游订阅套餐额度/用量，并自动在「Plan」渠道分组下创建专属转发渠道
- 📅 **用量报表** - 支持按日 / 周 / 月调度用量报表，通过通知渠道送达
- 💎 **模型广场** - 统一模型目录，展示价格、渠道覆盖、可用 Key 数、延迟和成功率等指标
- 🔃 **模型同步** - 自动与渠道同步可用模型列表，省心省力
- 📊 **Analytics 与 Evaluation** - 提供缓存概览、供应商 / 模型 / API Key 利用率、路由健康、延迟分布、语义缓存评估
- 🛠️ **Ops 与审计** - 提供遥测、配额、健康、系统、审计面板，以及管理面写操作审计链路
- 🧠 **语义缓存** - 为非流式和流式 OpenAI Chat / OpenAI Responses 文本请求提供基于 embedding 的语义缓存（流式命中可 SSE 重放）
- 🧭 **导航与页面配置** - 支持在设置页拖拽调整一级导航顺序和页面可见性，并持久化到服务端设置中
- 💾 **运行时状态持久化** - Auto 策略窗口和熔断器状态会持久化到数据库
- 🔗 **站点管理** - 管理上游中继平台，支持多账号、投射渠道、自动同步和自动签到
- 🌍 **代理池** - 命名代理配置，支持直连 / 系统 / 代理池 / 继承四种模式，并跟踪站点、账号和渠道的引用关系
- 🔁 **模型映射** - 全局模型名改写规则，支持精确、通配符和正则匹配，优先级排序和可选分组作用域
- ☁️ **WebDAV 云备份** - 基于 WebDAV 的自动云备份，支持定时调度、远程文件管理和一键恢复
- 🔑 **API 凭证档案** - 可复用的 Base URL + API Key 对，支持健康探测和 CLI 配置导出
- 📤 **CLI 配置导出** - 为 Claude Code、Codex、Gemini CLI、Cherry Studio、Kilo Code 生成即用配置片段
- 🎨 **优雅界面** - 简洁美观的 Web 管理面板，支持暗色模式、活跃热力图、分享快照和响应式移动布局
- 🗄️ **多数据库支持** - 支持 SQLite、MySQL、PostgreSQL，并可在三种数据库之间实时迁移

---

## 📚 文档目录

| 编号 | 页面 | 内容 |
|------|------|------|
| 01 | [安装](zh/01-安装.md) | Docker / Release / 源码构建，初始管理员设置 |
| 02 | [配置](zh/02-配置.md) | config.json、环境变量、SQLite 调优、数据库类型 |
| 03 | [角色与管理员](zh/03-角色与管理员.md) | admin / editor / viewer 角色与 WebAuthn/Passkey |
| 04 | [渠道管理](zh/04-渠道管理.md) | 渠道模板、Base URL、代理模式、请求改写、参数覆盖、Key 策略 |
| 05 | [分组管理](zh/05-分组管理.md) | 分组管理、负载均衡、模型发现与能力查询 |
| 06 | [模型广场](zh/06-模型广场.md) | 模型目录、定价、覆盖、能力双视图 |
| 07 | [Relay 端点](zh/07-Relay端点.md) | 公共 relay API、Zen 路由、模型映射、代理池、号池 |
| 08 | [分析](zh/08-分析.md) | 渠道×模型、用量明细、路由健康、延迟、评估、缓存 |
| 09 | [运维](zh/09-运维.md) | 遥测、配额、健康、维护、系统、审计 |
| 10 | [设置](zh/10-设置.md) | 设置卡片、语义缓存、数据库迁移、危险操作 |
| 11 | [站点管理](zh/11-站点管理.md) | 站点管理、WebDAV 备份、API 凭证、CLI 导出、通知中心 |
| 12 | [客户端接入](zh/12-客户端接入.md) | OpenAI SDK、Claude Code、Codex、CLI 导出 |
| 13 | [架构](zh/13-架构.md) | 分层架构、relay 数据流、Hub 适配器、时区、安全 |

---

## 📸 界面预览

> 截图于 2026-10-07，使用 v2.6.3-fix 隔离实例与合成演示数据。桌面视口为 1440 × 960，移动视口为 430 × 932，不含生产账号或凭据。

### 🖥️ 桌面端

<div align="center">
<table>
<tr>
<td align="center"><b>首页</b></td>
<td align="center"><b>渠道</b></td>
<td align="center"><b>分组</b></td>
</tr>
<tr>
<td><img src="../web/public/screenshot/desktop-home.png" alt="首页" width="400"></td>
<td><img src="../web/public/screenshot/desktop-channel.png" alt="渠道" width="400"></td>
<td><img src="../web/public/screenshot/desktop-group.png" alt="分组" width="400"></td>
</tr>
<tr>
<td align="center"><b>模型广场</b></td>
<td align="center"><b>日志</b></td>
<td align="center"><b>设置</b></td>
</tr>
<tr>
<td><img src="../web/public/screenshot/desktop-price.png" alt="模型广场" width="400"></td>
<td><img src="../web/public/screenshot/desktop-log.png" alt="日志" width="400"></td>
<td><img src="../web/public/screenshot/desktop-setting.png" alt="设置" width="400"></td>
</tr>
</table>
</div>

### 📱 移动端

<div align="center">
<table>
<tr>
<td align="center"><b>首页</b></td>
<td align="center"><b>渠道</b></td>
<td align="center"><b>分组</b></td>
<td align="center"><b>模型广场</b></td>
<td align="center"><b>日志</b></td>
<td align="center"><b>设置</b></td>
</tr>
<tr>
<td><img src="../web/public/screenshot/mobile-home.png" alt="移动端首页" width="140"></td>
<td><img src="../web/public/screenshot/mobile-channel.png" alt="移动端渠道" width="140"></td>
<td><img src="../web/public/screenshot/mobile-group.png" alt="移动端分组" width="140"></td>
<td><img src="../web/public/screenshot/mobile-price.png" alt="移动端模型广场" width="140"></td>
<td><img src="../web/public/screenshot/mobile-log.png" alt="移动端日志" width="140"></td>
<td><img src="../web/public/screenshot/mobile-setting.png" alt="移动端设置" width="140"></td>
</tr>
</table>
</div>

---

| → 下一页 | [安装](zh/01-安装.md) |
|----------|----------------------|
