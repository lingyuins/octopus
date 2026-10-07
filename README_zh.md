<div align="center">

<img src="web/public/logo.svg" alt="Octopus Logo" width="120" height="120">

### Octopus

**为个人打造的简单、美观、优雅的 LLM API 聚合与负载均衡服务**

简体中文 | [English](README.md) | [Changelog](CHANGELOG.md)

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
- 🧾 **API Key 治理** - 支持模型白名单、过期时间、费用上限、RPM / TPM 限额、按模型配额，以及 IP / CIDR 白名单
- 🔐 **角色化管理权限** - 内置 `admin`、`editor`、`viewer` 三种角色，并由服务端强制执行权限控制
- 🔑 **WebAuthn / Passkey 登录** — 通过 WebAuthn/Passkey 实现无密码登录和注册，支持可配置 RP 设置
- 🚨 **告警与通知中心** - 统一通知中心聚合系统事件、告警触发和套餐通知，支持 SSE 实时推送；告警规则支持错误率（含作用域 + 滑动窗口）、费用阈值、额度超限、渠道下线，通过 Webhook、Gotify、Email、Telegram、飞书、钉钉、企业微信、ntfy 八种渠道送达
- 📦 **套餐监控** - 跟踪上游订阅套餐额度/用量（Codex、MiMo、StepFun、SenseNova，以及 DeepSeek / Kimi / OpenRouter 等余额类厂商），并自动在「Plan」渠道分组下创建专属转发渠道
- 📅 **用量报表** - 支持按日 / 周 / 月调度用量报表，通过通知渠道送达
- 💎 **模型广场** - 统一模型目录，展示价格、渠道覆盖、可用 Key 数、延迟和成功率等指标，支持归一化去重多维筛选、可用端点与价格分类视图，同时保留创建 / 编辑 / 删除 / 刷新价格能力
- 🔃 **模型同步** - 自动与渠道同步可用模型列表，省心省力
- 📊 **Analytics 与 Evaluation** - 提供缓存概览、供应商 / 模型 / API Key 利用率、路由健康、延迟分布、语义缓存评估、供应商 Prompt Cache 分析，以及分组测试 / AI 路由入口
- 🛠️ **Ops 与审计** - 提供遥测、配额、健康、系统、审计面板，以及管理面写操作审计链路
- 🧠 **语义缓存** - 为非流式和流式 OpenAI Chat / OpenAI Responses 文本请求提供基于 embedding 的语义缓存（流式命中可 SSE 重放），并暴露运行态和成效指标
- 🧭 **导航与页面配置** - 支持在设置页拖拽调整一级导航顺序和页面可见性，并持久化到服务端设置中
- 💾 **运行时状态持久化** - Auto 策略窗口和熔断器状态会持久化到数据库
- 🔗 **站点管理** - 管理上游中继平台（New-API、One-API、One-Hub、Sub2API 等），支持多账号、投射渠道、自动同步和自动签到
- 🌍 **代理池** - 命名代理配置，支持直连 / 系统 / 代理池 / 继承四种模式，并跟踪站点、账号和渠道的引用关系
- 🔁 **模型映射** - 全局模型名改写规则，支持精确、通配符和正则匹配，优先级排序和可选分组作用域
- ☁️ **WebDAV 云备份** - 基于 WebDAV 的自动云备份，支持定时调度、远程文件管理和一键恢复
- 🔑 **API 凭证档案** - 可复用的 Base URL + API Key 对，支持健康探测和 CLI 配置导出
- 📤 **CLI 配置导出** - 为 Claude Code、Codex、Gemini CLI、Cherry Studio、Kilo Code 生成即用配置片段
- 🎨 **优雅界面** - 简洁美观的 Web 管理面板，支持暗色模式、活跃热力图、分享快照和响应式移动布局
- 🗄️ **多数据库支持** - 支持 SQLite、MySQL、PostgreSQL，并可在三种数据库之间实时迁移


## 🚀 快速开始

### 🐳 Docker 运行

以下示例使用 `latest`。如需固定版本，请在对应镜像发布后使用 `docker-compose.yml` 中记录的镜像标签。推送标签会启动发布流水线，请等待流水线完成后再拉取固定版本镜像。

直接运行

```bash
docker run -d --name octopus \
  --restart unless-stopped \
  -p 8080:8080 \
  -v octopus-data:/app/data \
  -e OCTOPUS_AUTH_JWT_SECRET="replace-with-a-long-random-secret" \
  lingyuins/octopus:latest
```

Windows Docker Desktop 推荐直接使用：

```powershell
docker run -d --name octopus `
  --restart unless-stopped `
  -p 8080:8080 `
  -v octopus-data:/app/data `
  -e OCTOPUS_AUTH_JWT_SECRET="replace-with-a-long-random-secret" `
  lingyuins/octopus:latest
```

或者使用 docker compose 运行

```yaml
services:
  octopus:
    image: lingyuins/octopus:latest
    container_name: octopus
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - octopus-data:/app/data
    environment:
      OCTOPUS_AUTH_JWT_SECRET: "replace-with-a-long-random-secret"

volumes:
  octopus-data:
```

然后执行：

```bash
docker compose up -d
```

注意：官方镜像默认以非 root 用户 `octopus` 运行，UID/GID 为 `1000`。上面的示例都为 `/app/data` 使用 Docker named volume，以避开大多数宿主机目录权限问题，尤其是 Linux 绑定挂载和 Windows Docker Desktop 场景。如果你有意把宿主机目录绑定挂载到 `/app/data`（例如 `./data:/app/data`），这个目录必须对 UID/GID `1000` 可写，否则启动时创建 `config.json` 或 `data.db` 会报 `permission denied`。

官方 Docker 镜像会在构建阶段重新编译前端，并把最新导出的管理界面嵌入 Go 二进制，因此容器内前端和对应发布版本保持一致。

> **🕐 时区：** 镜像默认使用 `Asia/Shanghai` 时区。若直接使用 `docker run`（而非 docker compose），需显式传入 `-e TZ=Asia/Shanghai` 或目标 IANA 时区（例如 `-e TZ=America/Los_Angeles`）。服务端日志时间戳、统计日期边界和前端时间展示均依赖容器的时区设置。

如果是从旧版前端升级，升级后浏览器若仍出现旧页面脚本报错，请清理一次站点数据 / Service Worker 缓存，确保加载到最新嵌入式前端资源。

> **🔗 反向代理部署：** Octopus 默认不信任任何代理，`c.ClientIP()` 只返回 TCP 直连地址。在 Docker bridge 网络或反代后，这个地址是网关（如 `172.17.0.1`）而非真实客户端 IP——中继日志、登录限流、API Key IP 白名单看到的都是网关地址。设置 `OCTOPUS_SERVER_TRUSTED_PROXIES` 为代理所在 CIDR（默认 Docker bridge 用 `172.17.0.0/16`）即可从 `X-Forwarded-For` 解析真实客户端 IP。


### 📦 从 Release 下载

从 [Releases](https://github.com/lingyuins/octopus/releases) 下载对应平台的二进制文件，然后运行：

```bash
./octopus start
```

### 🛠️ 源码运行

**环境要求：**
- Go 1.25.0
- Node.js 20+
- pnpm

```bash
# 克隆项目
git clone https://github.com/lingyuins/octopus.git
cd octopus
# 任何 Go 构建前必须执行：static/static.go 的 go:embed 要求该目录存在
#（目录被 gitignore，新克隆的仓库没有它）
mkdir -p static/out && touch static/out/.keep
# 可选：通过环境变量预置初始管理员账户
export OCTOPUS_INITIAL_ADMIN_USERNAME="admin"
export OCTOPUS_INITIAL_ADMIN_PASSWORD="change-this-password-long"
# 必需：下面两个密钥至少设置一个。security.encryption_key 与持久化 auth.jwt_secret
# 都为空时启动会被直接拒绝（用临时密钥派生 AES key 会导致重启后已加密数据无法解密）
export OCTOPUS_AUTH_JWT_SECRET="replace-with-a-long-random-secret"
export OCTOPUS_SECURITY_ENCRYPTION_KEY="replace-with-another-long-random-secret"
# 启动后端服务（嵌入前端产物前为 API-only 模式）
go run main.go start
```

> `static/out/` 这一步对源码构建是必需的：缺少该目录时，所有 `go build` / `go test` / `go run` 都会因 embed 指令失败。目录存在后（哪怕只有 `.keep`），后端即可编译并对外提供 API；把构建好的前端产物放到 `static/out/` 下（见下文）后，管理界面才可访问。

发布构建（`bash scripts/build.sh release`）使用 `--frozen-lockfile` 安装前端依赖；任一平台构建或产物准备失败都会终止。执行 `bash scripts/build_test.sh` 可在不编译二进制的情况下验证发布失败处理。

**构建嵌入式管理界面资源**

```bash
cd web && pnpm install && NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm build && cd ..
## 用新导出的产物替换之前生成的嵌入目录（static/out 是被 gitignore 的构建产物；
## web/out 原样保留，无需为再次构建或本地调试重新导出）
rm -rf static/out
mkdir -p static/out
cp -r web/out/. static/out/
# go:embed 对空目录会报错；如果 Next.js 导出了空的 _not-found，补一个占位文件
if [ -d web/out/_not-found ] && [ -z "$(ls -A web/out/_not-found)" ]; then
  touch static/out/_not-found/.keep
fi
# 重新启动后端，此时可直接访问嵌入式管理界面
go run main.go start
```

**开发模式**

```bash
cd web && pnpm install && NEXT_PUBLIC_API_BASE_URL="http://127.0.0.1:8080" NEXT_PUBLIC_APP_VERSION="$(git describe --tags --always 2>/dev/null || printf 'dev')" pnpm dev
## 新建终端，可选：通过环境变量自动创建初始管理员账户
export OCTOPUS_INITIAL_ADMIN_USERNAME="admin"
export OCTOPUS_INITIAL_ADMIN_PASSWORD="change-this-password-long"
## 必需：下面两个密钥至少设置一个（原因见上方快速开始说明）
export OCTOPUS_AUTH_JWT_SECRET="replace-with-a-long-random-secret"
export OCTOPUS_SECURITY_ENCRYPTION_KEY="replace-with-another-long-random-secret"
## 启动后端服务
go run main.go start
## 访问前端地址
http://localhost:3000
```

> **开发模式 CORS 提示**：开发态前端（`http://localhost:3000`）跨端口访问 `8080` 后端。CORS 白名单默认为空（= 拒绝所有跨域来源），若浏览器拦截了 API 请求，请在 `设置 → 系统 → CORS` 中加入 `http://localhost:3000`（或 `*`）。

### 🔐 初始管理员设置

首次启动时，可以通过以下任一方式完成管理员初始化：

- 设置 `OCTOPUS_INITIAL_ADMIN_USERNAME` 和 `OCTOPUS_INITIAL_ADMIN_PASSWORD`，在启动时自动创建初始管理员账户
- 或在首次访问 Web UI 时，在引导向导页面中手动创建初始管理员账户

> ⚠️ **安全提示**：初始管理员密码长度必须至少为 12 个字符。
>
> ⚠️ **安全提示**：必须配置持久化 `auth.jwt_secret` 或 `security.encryption_key`，两者均未配置时会拒绝启动，避免加密凭据丢失。若只配置加密密钥，JWT 密钥仍为临时值，服务重启后已有登录 token 会失效。

### 👥 管理员角色

管理 API 和内嵌 Web 管理界面内置三种角色：

- `admin`：完整权限，包括用户管理
- `editor`：可写的运维权限，包括渠道、分组、设置、API Key、日志、告警和 AI 路由
- `viewer`：只读权限

权限校验由服务端执行，服务端会按当前存储的角色判权，而不是只信任 JWT 中的角色声明。

### 📝 配置文件

配置文件默认位于 `data/config.json`，首次启动时自动生成。

**完整配置示例：**

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "database": {
    "type": "sqlite",
    "path": "data/data.db"
  },
  "log": {
    "level": "info"
  },
  "auth": {
    "jwt_secret": "replace-with-a-long-random-secret"
  },
  "security": {
    "encryption_key": "replace-with-another-long-random-secret"
  }
}
```

大多数运行时调优项不存放在 `config.json` 中。重试策略、熔断阈值、Auto 策略调优、日志保留周期、对外 API 基础地址、AI 路由服务配置、语义缓存开关、WebDAV 备份、代理池、模型映射规则等，都通过设置页 / 管理 API 动态写入数据库。

**配置项说明：**

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `server.host` | 监听地址 | `0.0.0.0` |
| `server.port` | 服务端口 | `8080` |
| `server.trusted_proxies` | 信任的反向代理 CIDR/IP 列表（逗号分隔），用于从 `X-Forwarded-For` 解析真实客户端 IP。空值=不信任任何代理（安全默认，`c.ClientIP()` 返回 TCP 直连地址）；`*`=信任全部（仅开发用，有 XFF 伪造风险） | 空 |
| `database.type` | 数据库类型 | `sqlite` |
| `database.path` | 数据库连接地址 | `data/data.db` |
| `log.level` | 日志级别 | `info` |
| `auth.jwt_secret` | JWT 签名密钥 | 空（未设置时启动生成临时密钥） |
| `security.encryption_key` | 敏感数据存储加密密钥（凭证档案、站点密码等） | 空（回退到持久化 JWT 密钥；两者均未配置时拒绝启动） |
| `relay.max_json_body_bytes` | JSON 请求体最大大小 | `67108864`（64 MB） |
| `relay.max_multipart_body_bytes` | Multipart 请求体最大大小 | `67108864`（64 MB） |

> 💡 **提示**：在生产环境运行 Octopus 前，请设置 `OCTOPUS_AUTH_JWT_SECRET` 或 `auth.jwt_secret`，这样登录 token 才能在服务重启后继续有效。

**数据库配置：**

支持三种数据库：

| 类型 | `database.type` | `database.path` 格式 |
|------|-----------------|---------------------|
| SQLite | `sqlite` | `data/data.db` |
| MySQL | `mysql` | `user:password@tcp(host:port)/dbname` |
| PostgreSQL | `postgres` | `postgresql://user:password@host:port/dbname?sslmode=disable` |

**MySQL 配置示例：**

```json
{
  "database": {
    "type": "mysql",
    "path": "root:password@tcp(127.0.0.1:3306)/octopus"
  }
}
```

**PostgreSQL 配置示例：**

```json
{
  "database": {
    "type": "postgres",
    "path": "postgresql://user:password@localhost:5432/octopus?sslmode=disable"
  }
}
```

> 💡 **提示**：MySQL 和 PostgreSQL 需要先手动创建数据库，程序会自动创建表结构。

**环境变量：**

所有配置项均可通过环境变量覆盖，格式为 `OCTOPUS_` + 配置路径（用 `_` 连接）：

| 环境变量 | 对应配置项 |
|----------|-----------|
| `OCTOPUS_SERVER_PORT` | `server.port` |
| `OCTOPUS_SERVER_HOST` | `server.host` |
| `OCTOPUS_SERVER_TRUSTED_PROXIES` | `server.trusted_proxies` |
| `OCTOPUS_DATABASE_TYPE` | `database.type` |
| `OCTOPUS_DATABASE_PATH` | `database.path` |
| `OCTOPUS_DATA_DIR` | 在未显式设置 `database.path` 时，`config.json` 和 SQLite 数据库的默认目录 |
| `OCTOPUS_LOG_LEVEL` | `log.level` |
| `OCTOPUS_AUTH_JWT_SECRET` | `auth.jwt_secret` |
| `OCTOPUS_SECURITY_ENCRYPTION_KEY` | `security.encryption_key` |
| `OCTOPUS_INITIAL_ADMIN_USERNAME` | 启动时自动创建初始管理员用户名 |
| `OCTOPUS_INITIAL_ADMIN_PASSWORD` | 启动时自动创建初始管理员密码 |
| `OCTOPUS_GITHUB_PAT` | 用于获取最新版本时的速率限制(可选) |
| `OCTOPUS_RELAY_MAX_SSE_EVENT_SIZE` | 最大 SSE 事件大小(可选) |


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
<td><img src="web/public/screenshot/desktop-home.png" alt="首页" width="400"></td>
<td><img src="web/public/screenshot/desktop-channel.png" alt="渠道" width="400"></td>
<td><img src="web/public/screenshot/desktop-group.png" alt="分组" width="400"></td>
</tr>
<tr>
<td align="center"><b>模型广场</b></td>
<td align="center"><b>日志</b></td>
<td align="center"><b>设置</b></td>
</tr>
<tr>
<td><img src="web/public/screenshot/desktop-price.png" alt="模型广场" width="400"></td>
<td><img src="web/public/screenshot/desktop-log.png" alt="日志" width="400"></td>
<td><img src="web/public/screenshot/desktop-setting.png" alt="设置" width="400"></td>
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
<td><img src="web/public/screenshot/mobile-home.png" alt="移动端首页" width="140"></td>
<td><img src="web/public/screenshot/mobile-channel.png" alt="移动端渠道" width="140"></td>
<td><img src="web/public/screenshot/mobile-group.png" alt="移动端分组" width="140"></td>
<td><img src="web/public/screenshot/mobile-price.png" alt="移动端模型广场" width="140"></td>
<td><img src="web/public/screenshot/mobile-log.png" alt="移动端日志" width="140"></td>
<td><img src="web/public/screenshot/mobile-setting.png" alt="移动端设置" width="140"></td>
</tr>
</table>
</div>


## 📖 功能说明

### 🧭 管理台模块

当前内嵌管理台包含以下一级模块：

| 模块 | 作用 |
|------|------|
| Home | 版本信息、运行状态、高层摘要、趋势图、GitHub 风格活跃热力图和排行榜 |
| Hub | 上游中继平台管理，包含站点（多账号卡片，内联余额/同步/签到状态、归档/恢复、批量编辑、AllAPIHub/MetAPI 批量导入）、站点渠道（投射渠道绑定）、自动化（自动同步与自动签到间隔）、额度（套餐余额图表）和 TokenPlan（Token 套餐监控）等页签 |
| Channel | 上游渠道、Key、Header、同步、延迟探测、代理模式和请求改写配置 |
| 号池 | 面向 relay 调度的号池管理：号池列表支持搜索与新建/编辑（名称、描述、策略、默认并发、冷却、启用）并带删除二次确认；详情页支持按关键字 / 平台 / 状态筛选账号、OAuth 账号授权、批量操作和凭据导入导出 |
| Group | 模型路由、负载均衡、会话保持、分组测试、AI 路由、分组思考模式（自动 / 关闭 / 开启）、端点供应商、Zashboard 风格可折叠分组列表和 CC Switch 深链接 |
| Model Market | 模型目录（市场 / 可用端点 / 价格分类三视图）、自定义价格、渠道覆盖、可用 Key 数、延迟、成功率摘要、归一化去重多维筛选，以及兜底定价与峰谷计费规则 |
| Analytics | 渠道×模型（默认）、用量拆分、路由健康、延迟分布、评估中心、缓存（语义 + 供应商 Prompt Cache）和分享快照 |
| Log | 双视图：Relay 请求历史，详情支持分组 / 请求体切换、模型与渠道候选状态和响应并排展示、展开尝试诊断，以及 Token 使用和费用记录；另有错误日志（Error Log）视图查看后端记录的错误 |
| Notification | 统一通知中心，分组涵盖消息（收件箱 / 已归档）、告警（规则 / 历史）、送达（渠道 / 策略 / 偏好）和报表（调度 / 历史）。告警规则、通知渠道（Webhook、Gotify、Email、Telegram、飞书、钉钉、企业微信、ntfy）和用量报表调度都在这里 |
| Ops | 遥测（Hero 指标、P95 延迟、供应商健康、Prompt Cache 分析）、配额、健康、维护（重试 / 熔断 / 响应过滤）、系统和审计轨迹 |
| APIKey | API Key 创建、编辑、删除，模型白名单、过期时间、费用上限、RPM / TPM 配额、IP 白名单和按模型配额 |
| Setting | 版本更新信息、外观与导航偏好（排序 + 可见性）、AI 路由服务池、语义缓存、日志保留、系统选项（Public API Base URL、代理、CORS）、LLM/价格同步节奏、数据库备份与实时迁移、Redis 缓存后端、WebDAV 备份、WebAuthn/Passkey、模型名归一化、号池调度设置、代理池入口和危险操作 |
| User | 管理员用户和角色管理 |

此外，以下功能可通过应用外壳工具栏或其他模块内访问：

| 功能 | 作用 |
|------|------|
| 代理池 | 命名代理配置的增删改查、连通性测试和引用树追踪 |
| 模型映射 | 全局模型名改写规则，支持精确 / 通配符 / 正则匹配、优先级和分组作用域 |
| API 凭证档案 | 可复用的 Base URL + API Key 对，支持健康探测和 CLI 导出 |

### 📡 渠道管理

渠道是连接 LLM 供应商的基础配置单元。

**渠道模板：**

UI 提供内置渠道模板用于快速创建（以 `web/src/components/modules/channel/templates.ts` 为准）：OpenAI（创建的是 OpenAI Responses 类型渠道）、Anthropic、Gemini、DeepSeek、OpenRouter、SiliconFlow、Volcengine 和 MiMo。

**Base URL 说明：**

程序会根据渠道类型自动补全 API 路径，您只需填写基础 URL 即可：

| 渠道类型 | 自动补全路径 | 填写 URL | 完整请求地址示例 |
|----------|-------------|----------|-----------------|
| OpenAI Chat | `/chat/completions` | `https://api.openai.com/v1` | `https://api.openai.com/v1/chat/completions` |
| OpenAI Responses | `/responses` | `https://api.openai.com/v1` | `https://api.openai.com/v1/responses` |
| OpenAI Embeddings | `/embeddings` | `https://api.openai.com/v1` | `https://api.openai.com/v1/embeddings` |
| OpenAI Images | `/images/generations`、`/images/edits`、`/images/variations` | `https://api.openai.com/v1` | `https://api.openai.com/v1/images/generations` |
| Anthropic | `/messages` | `https://api.anthropic.com/v1` | `https://api.anthropic.com/v1/messages` |
| Gemini | `/models/:model:generateContent` | `https://generativelanguage.googleapis.com/v1beta` | `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent` |
| Volcengine | `/responses` | `https://ark.cn-beijing.volces.com/api/v3` | `https://ark.cn-beijing.volces.com/api/v3/responses` |
| MiMo Chat | `/chat/completions` | `https://api.xiaomimimo.com/v1` | `https://api.xiaomimimo.com/v1/chat/completions` |

> 💡 **提示**：Base URL 现在支持 `自动识别` 和 `自定义` 两种模式。`自动识别` 会按渠道类型自动补版本后缀，`自定义` 会保持你填写的 URL 原样。

**代理模式：**

每个渠道可配置代理模式：

| 模式 | 说明 |
|------|------|
| `direct` | 不使用代理，直接连接 |
| `system` | 使用系统代理设置 |
| `pool` | 从命名代理池中选择 |
| `inherit` | 从父站点或账号继承代理 |

**请求改写配置：**

按渠道配置请求改写以兼容上游：

| 配置 | 说明 |
|------|------|
| `preserve` | 不改写请求体，原样转发 |
| `openai_chat_compat` | 剥离不兼容字段以适配标准 OpenAI Chat 格式 |
| `codex` | Codex 专用 Header 整形和工具/系统消息策略 |

**参数覆盖：**

每个渠道支持 `param_override` JSON 配置，可在发往上游供应商的出站请求中注入或覆盖特定参数，实现按渠道的参数自定义而无需修改客户端请求。

Header 和消息策略：

| 策略 | 选项 | 说明 |
|------|------|------|
| Header 配置 | `none`、`codex` | Codex 专用 Header 整形 |
| Tool 角色 | `keep`、`stringify_to_user` | 如何处理 tool 角色消息 |
| System 消息 | `keep`、`merge` | 如何处理系统消息 |

**跳过模型可用性测试：**

每个渠道提供 `skip_model_test` 开关（issue #98）。开启后该渠道不参与分组/模型可用性探测——适用于因低字节探测请求而扣额度或封禁账号的上游。被跳过的渠道会在测试日志中记为一次不通过的尝试并附上明确原因，而不会真正向上游发请求。

**Key 选择策略：**

当渠道配置了多个 Key 时，全局 `key_selection_strategy` 设置控制每次请求挑选哪个 Key：

| 策略 | 说明 |
|------|------|
| `cost` | 默认。优先选择成本最低的 Key |
| `availability` | 优先选择可用度评分更高的 Key（按错误类型加权，支持基于时间的惰性恢复） |
| `speed` | 优先选择近期 TPS（每秒 token 数）吞吐更高的 Key（issue #140） |
| `priority` | 优先选择按 Key 优先级排序更高的 Key |

**定时 Key 可用性巡检：**

后台任务（issue #142）按 `key_health_check_interval`（分钟）配置的间隔定期探测所有渠道 Key。失败的 Key 会触发通知并将受影响的渠道标灰，让降级的上游一目了然。

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

**Zen 直连模型路由：**

以 `zen/<model>` 前缀发起的请求会绕过分组模型映射，直接路由到上游模型。Octopus 会根据模型名进行智能渠道类型检测（如 Claude → Anthropic，Gemini → Gemini，GPT → OpenAI）。

**Response ID 亲和性：**

对于 OpenAI Responses API，引用同一 response ID 的后续请求会自动路由到同一上游渠道，以保持对话连续性。

**模型映射：**

全局模型名改写规则在 relay 管线中于分组解析之前生效。规则支持精确、通配符（glob）和正则匹配，带优先级排序和可选分组作用域。

---

### 🔍 模型发现与能力查询

Octopus 提供多层模型可见性：

#### `/v1/models` — 兼容模型列表

返回所有至少具有一个启用渠道的模型名称。兼容 OpenAI SDK。

这是最宽泛的视图 — 模型出现在这里，表示 Octopus 有渠道*声明*了它。

#### `/v1/models?endpoint=<type>` — 按端点类型过滤

按**声明的端点类型**缩小列表：

- `?endpoint=chat` — 对话模型（chat / responses / messages / deepseek / mimo）
- `?endpoint=embeddings` — 嵌入模型
- `?endpoint=image_generation` — 图像生成模型
- `?endpoint=music_generation` — 音乐生成模型
- …… 同样适用于 `audio_speech`、`audio_transcription`、`video_generation`、`search`、`rerank`、`moderations`

省略 `endpoint` 或设为 `*` 时返回全部模型。

> 部分端点之间的边界并非绝对。**对话族**（`chat`、`responses`、`messages`、`deepseek`、`mimo`）可以在 `endpoint` 过滤器中互相可见，因为 Octopus 可以透明桥接这些格式。

#### `GET /api/v1/model/capabilities` — 声明的能力表（管理 API）

仅管理端可用的端点，返回每个可路由模型的**聚合能力视图**：

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "name": "gpt-4o",
      "endpoints": ["chat"],
      "conversation": true,
      "available": true
    },
    {
      "name": "music-2.6",
      "endpoints": ["music_generation"],
      "conversation": false,
      "available": true
    }
  ]
}
```

| 字段 | 含义 |
|------|------|
| `name` | 对客户端公开的模型名称 |
| `endpoints` | 模型声明的端点类型（去重并排序） |
| `conversation` | 是否属于对话族 |
| `available` | 是否至少有一个启用的渠道 |

这是**声明的**能力 — 即你的 `Group` 配置所描述的能力。实际可路由的能力可能更窄；见下方 `*` group 行为。

#### `*` Group 语义

端点类型为 `*`（EndpointTypeAll）的分组是一个**万能通行证**：它可以被任意端点类型选中，包括 `chat`、`embeddings`、`image_generation` 等。

然而，**万能选中并不意味着组内每个条目都真正支持该端点**。对于非对话端点（image / video / music / audio / search / rerank / moderation），relay 层现在会在进入负载均衡器之前过滤 `*` 组条目：

- 仅保留其渠道类型或模型名暗示支持当前端点的条目。
- 如果没有条目能通过过滤，请求直接返回 `404 model not found`，而不是盲目尝试不兼容的渠道。
- 对话端点（`chat`、`responses`、`messages`、`deepseek`、`mimo`）**不受**此过滤影响。

> **提示：** 如果你在 `/v1/models` 或 `/api/v1/model/capabilities` 中看到某个模型，但对特定端点仍然返回 `model not found`，请检查 `*` 组中的条目是否真的支持该端点 — relay 级收窄可能已将它们全部过滤掉。

---

### 📁 分组管理

分组用于将多个渠道聚合为一个统一的对外模型名称。

**核心概念：**

- **分组名称** 即程序对外暴露的模型名称
- 调用 API 时，将请求中的 `model` 参数设置为分组名称即可
- **首字超时**：单位秒，仅对流式响应生效，`0` 表示不限制
- **会话保持**：单位秒，在设定时间窗口内，同一 API Key + 模型会优先复用上次成功的渠道，`0` 表示禁用
- **Condition (JSON)**：可选的 AND 条件规则，当前只在主 LLM relay 路径里生效；内置请求上下文目前包含 `model`、`api_key_id`、`hour`
- **端点供应商（Endpoint Provider）**：按端点类型进行供应商感知的请求改写。对话供应商（`openai`、`deepseek`、`mimo`、`siliconflow`、`newapi`）会剥离不兼容的推理字段；音乐供应商（`newapi`、`minimax`）改写请求体和路径；视频供应商（`agnes`）改写上流路径；语音合成供应商（`mimo`）转换请求格式和路径
- **出站格式（Outbound Format）**：控制跨格式适配器回退。`""`（自动）、`chat`、`responses` 仅调整 Chat/Responses 的尝试顺序；`chat_only` 和 `responses_only` 会完全禁用回退——适用于用 400/404 拒绝另一格式的上游（如公益站）
- **思考模式**：在分组编辑器的高级配置中设置 `thinking_mode`。`auto`（默认）保持客户端 / 上游行为；`off` / `on` 覆盖客户端推理参数，客户端未传参时也生效。适配器按 DeepSeek/MiMo、OpenAI Chat/Responses（含 Codex）、Anthropic、Gemini、火山引擎各自协议发送控制字段。通过通用域名代理 DeepSeek/MiMo 时，请选择对应 API 分类或端点供应商。需要上游模型支持；原始透传 / 原始穿透保持请求体不变，不应用此策略。此功能不新增 FIM 或 `/v1/completions` 端点。
- **Key 冷却**：限速冷却按 `(keyID, model)` 粒度追踪（issue #94），单个模型的 429 不再拖累同 Key 的其他模型。单渠道重试次数可设为 `0`（只试一次即换渠道）；最大总尝试次数配额只统计真实发往上游的转发，冷却/熔断跳过不占用配额

**负载均衡模式：**

| 模式 | 说明 |
|------|------|
| 🔄 **轮询** | 每次请求依次切换到下一个渠道 |
| 🎲 **随机** | 每次请求随机选择一个可用渠道 |
| 🛡️ **故障转移** | 优先使用高优先级渠道，仅当其故障时才切换到低优先级渠道 |
| ⚖️ **加权分配** | 按权重从高到低排序后依次尝试渠道 |
| 🤖 **智能选择** | 优先探索样本不足的候选，样本充足后按时间窗口内成功率优选 |

**Auto 智能策略默认值：**

- **最小样本数**：`10`
- **时间窗口**：`300` 秒
- **滑动窗口大小**：每个渠道-模型对保留 `100` 条记录
- **延迟权重**：`30`
- 当候选未达到最小样本数时，系统优先进行探索
- 候选都完成探索后，系统按成功率排序；成功率相同时，再按样本量、权重、优先级和延迟调优兜底
- Auto 策略窗口会在启动时从数据库恢复，并在定时任务和优雅退出时持久化

**AI 路由行为：**

- 在路由页点击 **AI路由** 时，系统会把全部模型发送给 AI，批量生成整张路由表
- 遇到同名已有分组时，只会追加缺失的路由项，不会清空或替换已有分组
- 在分组编辑弹窗点击 **AI补全当前分组** 时，系统会把全部模型发送给 AI，并只向当前分组追加匹配到的路由项
- 原先的 "AI路由目标分组" 设置现在只作为单分组兼容模式下的默认目标分组使用
- AI 路由任务具有持久化能力，支持心跳、进度追踪、批次管理和中断恢复

**CC Switch 集成：**

分组工具栏包含 CC Switch 深链接生成器，可为下列目标应用生成供应商导入链接：Claude Code、Codex、Gemini、OpenCode 和 OpenClaw。对于 Claude Code，支持将 Haiku / Sonnet / Opus 模型映射到指定路由分组。

> 💡 **示例**：创建分组名称为 `gpt-4o`，将多个供应商的 GPT-4o 渠道加入该分组，即可通过统一的 `model: gpt-4o` 访问所有渠道。

---

### 💎 模型广场与价格

`Model` 路由（模型广场）提供可在工具栏切换的视图：**市场**（价格与覆盖卡片）、**可用端点**（端点 → 模型分组）和**价格分类**（兜底定价规则 + 峰谷计费）。

**市场视图每张卡片整合的数据：**

- LLM 价格目录中的自定义价格或同步价格（输入 / 输出 / 缓存读 / 缓存写）
- 渠道与模型关系中的覆盖渠道数、可用 Key 数
- 模型统计中的平均延迟、成功率、成功次数、失败次数
- 命中峰谷计费规则的模型会显示峰谷计费徽标（目录价为高峰价，空闲价打折）

**市场视图的多维筛选：**

- 与工具栏搜索框共用的名称搜索
- 能力筛选（chat、embeddings、rerank 等）——对话族端点计入 chat 能力
- 按模型名推断的厂商筛选
- 价格筛选：全部 / 有价 / 免费
- 归一化去重：合并同一基础模型的不同命名变体（如 `kimi-k2.5`、`moonshotai/kimi-k2.5`、`dmxapi-kimi-k2.5-cc`）。规则来自设置页 `Normalize` 卡片（路由前缀、功能性后缀、显式变体→基准名映射），并可通过专属设置默认开启
- 可折叠的筛选条实时展示结果数量（可见 / 总数）、生效筛选条件数，以及一键重置（同时清空搜索、能力、厂商、价格与去重）

**顶部摘要指标：**

工具栏摘要区展示四项指标以及价格更新时间与刷新价格按钮。这些数字来自 market 接口对**全量**数据计算的 summary——**不随**搜索或筛选变化（筛选只影响卡片列表）。开启默认去重设置后，market 接口本身会在服务端聚合命名变体，卡片与摘要统计已按合并后的条目计算：

| 指标 | 含义 |
|------|------|
| 模型数 | 市场中聚合后的模型总数 |
| 覆盖渠道数 | 全市场渠道对模型的覆盖总数 |
| 唯一渠道数 | 全市场涉及的去重渠道数 |
| 平均延迟 | 全市场按请求统计加权后的平均延迟 |

**卡片与对话框：**

- 响应式虚拟化卡片（桌面 grid / list / compact 布局，移动端单列列表），支持按成功率或请求量排序
- 卡片可展开查看价格对（输入 / 缓存读、输出 / 缓存写，支持人民币换算）、运行指标与逐渠道明细（启用状态与可用 Key 数）；渠道标签过多时折叠为 `+N`
- 标准编辑对话框带严格价格校验（非负十进制数，非法字段内联高亮而非静默清零）与删除确认对话框

**可用端点视图：**

从有效路由分组聚合并反转为端点 → 模型分组。对话族端点（chat / deepseek / mimo / responses / messages / 自动）合并为单一的「对话」分组并排在最前；未声明端点的模型归入自动（`*`）分组。该视图与市场视图共用工具栏搜索框——搜索同时匹配端点名称 / 标签与模型名，并强制展开全部分组、显示所有命中芯片。每个分组卡片可折叠，带模型数量与厂商图标芯片，模型较多时可通过「展开更多」控制。

**价格分类视图：**

- 为没有精确价格的模型提供按规则的兜底定价；规则按优先级（sort order）顺序匹配，首个命中生效
- 规则表格：名称、规则类型（exact / prefix / contains）、规则值、四项价格、优先级与启用徽标
- 创建 / 编辑对话框带校验：名称与规则值必填、价格必须为非负有限数、优先级必须为整数；错误在首次提交后展示，随输入修正即时消失
- 峰谷计费区（当前针对 DeepSeek）：空闲倍率、周末空闲开关，以及两段北京时间窗口（留空表示该窗口关闭；两窗全关 = 全天空闲；窗口重叠后端允许，仅提示不阻断）
- 规则变更会使市场价格缓存失效；该视图不提供搜索框

**查询门控：**

切换页面时视图保持挂载（keep-alive），因此所有查询都做了门控：market 与能力查询只在模型模块激活且对应视图选中时才运行，切换视图即停止上一个视图的查询。

**数据来源：**

- 系统会定期从 [models.dev](https://github.com/sst/models.dev) 同步更新模型价格数据
- 当创建渠道或同步渠道模型时，如果某个模型还不在本地目录里，Octopus 会自动创建本地价格记录，确保后续仍可手动维护价格
- 也支持手动创建 models.dev 中已存在的模型，用于自定义价格

**价格优先级：**

| 优先级 | 来源 | 说明 |
|:------:|------|------|
| 🥇 高 | 本页面 | 用户在模型广场页面设置的价格 |
| 🥈 低 | models.dev | 自动同步的默认价格 |

> 💡 **提示**：如需覆盖某个模型的默认价格，只需在模型广场页面为其设置自定义价格即可。

**页面仍保留的操作：**

- 创建自定义模型价格
- 编辑已有模型的输入 / 输出 / 缓存价格
- 删除自定义模型条目
- 在工具栏摘要区手动刷新上游价格
- 在设置页 `LLM Sync` 卡片里继续维护定时刷新策略

---

### 📈 Analytics

Analytics 是偏只读的分析模块，默认 Tab 为 **渠道×模型**，让最关注的数据先呈现：

| 页签 | 展示内容 |
|------|----------|
| 渠道×模型 | 渠道×模型用量矩阵、用量分布占比图（按模型 / 按渠道×模型 Top-N，可切换费用/次数/Token） |
| 用量明细 | 按供应商、模型、API Key 的用量拆分（由"利用率"重命名，金额为空时显示"无计费数据"提示） |
| 路由健康 | 每个分组的健康分、启用 / 禁用项数量、近期失败压力 |
| 延迟 | 请求延迟指标（Avg、P50、P95、P99）、首字延迟（FTUT）指标和延迟分布直方图 |
| 评估 | 分组可用性、AI 路由进度、分组测试进度、语义缓存成效 |
| 缓存 | 语义缓存成效和供应商侧 Prompt Cache 分析（缓存率、复用比、每供应商预估节省费用） |

**时间范围：** `1d`、`7d`、`30d`、`90d`、`ytd`、`all`

`/api/v1/analytics/overview` 这个概览接口仍然保留，但当前 UI 中这些摘要指标的主要入口已经迁移到首页。首页现在额外承载独立的 `7d / 30d / 90d` 概览范围切换，以及 Hero 摘要、趋势图、GitHub 风格活跃热力图和排行榜。

`Evaluation` 不会复制完整的分组页和设置页，而是作为轻量入口，把分组测试、AI 路由和语义缓存评估串起来。

**分享快照：**

Analytics 页面包含分享按钮，可生成当前分析状态的 PNG 视觉快照，支持下载或复制到剪贴板。快照包含关键统计数据（请求数、Token 数、费用、供应商数、缓存命中率）和时间戳。

---

### 🛠️ Ops

Ops 模块面向运行态诊断和运维视角，当前包含：

| 页签 | 展示内容 |
|------|----------|
| 遥测 | Hero 指标（运行时间、总请求数、平均延迟、错误率、活跃连接、内存使用）、P95 延迟、吞吐量 RPS、数据库健康、会话与配额活跃度、语义缓存快照、供应商健康表（支持列排序 + 迷你条形图） |
| 配额 | API Key 在 RPM、TPM、费用上限、按模型配额上的整体姿态，合并总 Token、成功率与"查看密钥详情"跳转 |
| 健康 | 数据库连通性、缓存就绪状态、任务运行状态、近期错误量、异常分组（含跳转至分析中心 → 路由健康） |
| 维护 | 可操作运行时调优：重试、熔断器、响应过滤三组设置合并至同一页签（已从设置页迁出） |
| 系统 | 构建信息、数据库类型、Public API Base URL、代理、保留周期、AI 路由模式和服务列表 |
| 审计 | 管理面写操作的分页审计日志 |

**供应商 Prompt Cache 分析：**

Telemetry 标签页包含供应商侧 Prompt Cache 监控，追踪上游供应商的 Prompt 缓存有效性：缓存率、缓存复用比、缓存读/写 Token 数、每渠道预估节省费用和 24 小时缓存趋势图。这与语义缓存独立。

**审计范围：**

- 覆盖已纳入白名单的管理面写接口，例如 channel / group / model / setting / API key / alert / user 变更、AI 路由生成、日志清理、价格刷新、导入、自更新等
- 不记录公共 `/v1/...` relay 流量

---

### ⚙️ 设置

系统全局配置项。

**统计保存周期（分钟）：**

由于程序涉及大量统计项目，若每次请求都直接写入数据库会影响读写性能。因此程序采用以下策略：

- 统计数据先保存在 **内存** 中
- 按设定的周期 **定期批量写入** 数据库
- 中继负载均衡运行时状态也采用同样的周期持久化方式

**运行时状态持久化：**

- Auto 策略窗口会在启动时从数据库恢复
- 熔断器状态会在启动时从数据库恢复
- 二者都会按统计保存周期定时落库
- 二者也会在优雅退出时主动保存

**当前设置页的重点卡片**（权威清单见 `web/src/components/modules/setting/index.tsx`）：

| 卡片 | 作用 |
|------|------|
| Info | 当前版本、最新发布检查、前后端版本不一致检测、站内自更新入口（含版本不匹配通知） |
| Appearance | 主题、语言、告警通知语言，一级导航顺序拖拽偏好，以及各页面可见性开关 |
| AI Route | 单分组兼容默认目标、超时、并发度、服务池配置 |
| Auto Strategy | Auto 策略调优（最小样本数、时间窗口、滑动窗口大小、延迟权重） |
| Account | 登录会话/账户偏好和应用时区选择 |
| Semantic Cache | 开关、TTL、相似度阈值、最大条目数、embedding Base URL / API Key / 模型 / 超时 |
| Log | 日志保留（按时长和按数量）和日志级别 |
| System | Public API Base URL、代理、CORS 白名单（标签式管理）和统计落库周期 |
| LLM Sync | 上游模型同步和价格刷新节奏 |
| Backup | 数据库导出、导入和实时数据库迁移（SQLite / MySQL / PostgreSQL），支持连接测试和按表行数结果展示 |
| Redis | 可选 Redis 缓存后端配置：连接设置、连接测试、保存（重启生效）。将统计、运行时状态、限流/冷却和渠道延迟探测卸载到 Redis，适用于低内存主机和多实例横向扩展 |
| WebDAV Backup | WebDAV 云备份配置：连接设置、自动备份间隔、最大备份保留数、手动触发、远程文件列表、恢复和删除 |
| WebAuthn / Passkey | RP ID、RP 展示名、允许的 Origin 配置 |
| 归一化（Normalize） | 模型名归一化规则：路由前缀、功能后缀和显式变体→基准名映射（运行时可配置，支持离线 AI 辅助归一化工作流） |
| 号池（Pool） | 号池调度调优：健康巡检、分层调度、粘性会话逃逸、权重重置等相关设置 |
| 代理池（Proxy Pool） | 打开共享代理池管理弹窗（无内嵌面板） |

> **说明：** 以下设置项已迁移到更相关的模块（issue #87）：
> - **重试 / 熔断器 / 响应过滤** → `运维中心 → 维护` 页签
> - **站点自动化** → `Hub → 自动化` 页签
> - **清理不可用模型 / 删除全部路由分组** → `分组` 页"维护"下拉按钮

**语义缓存的真实生效条件：**

- 作用于非流式和流式 OpenAI Chat / OpenAI Responses 文本请求
- 流式缓存命中会从 SSE 会话缓冲区重放，并支持稳定的流会话恢复
- 缓存命名空间按 `api_key_id + endpoint_family + requested_model` 隔离
- 即使开关打开，只要 embedding 客户端没有配完整，或查询 / 写入 embedding 失败，也会自动旁路，不阻断正常转发
- 运行态与成效可同时在 `Analytics -> Cache` 和 `Ops -> Telemetry` 中查看
- 运行配置未变更时，缓存条目会在刷新中保留

**数据库实时迁移：**

Backup 设置卡片包含超越简单导出/导入的实时数据库迁移功能：

- 目标数据库类型：SQLite、MySQL、PostgreSQL
- 迁移前连接测试
- 可选包含日志和统计数据
- 迁移结果展示，含按表行数统计
- 迁移后重启提醒（后端在重启前继续使用旧数据库）

**设置页危险操作：**

- 设置页新增了 **删除全部路由分组**
- 执行前要求二次确认
- 操作会删除全部分组和分组项，并把单分组 AI 路由默认目标分组重置为 `0`，避免残留悬挂引用

**设置卡片排序：**

设置页支持对卡片区域进行拖拽排序，排序结果会持久化到本地存储。提供"恢复默认"按钮。

> ⚠️ **重要提示**：退出程序时，请使用正常的关闭方式（如 `Ctrl+C` 或发送 `SIGTERM` 信号），以确保内存中的统计数据能正确写入数据库。**请勿使用 `kill -9` 等强制终止方式**，否则可能导致统计数据丢失。

---

### 🔗 站点管理（Hub → 站点）

Hub 模块的**站点**页签将上游中继平台作为一等实体管理。站点代表 New-API、One-API、One-Hub、Done-Hub、Sub2API、OpenAI、Claude、Gemini、SAPI 等平台。每个站点渲染为多账号卡片，内联展示余额、同步状态和签到状态，替代了此前的多页签 Hub 布局。

**功能特性：**

- 每个站点支持多账号，凭证类型包括用户名/密码、access_token 或 api_key
- 按可配置间隔自动同步渠道、令牌和模型
- 自动签到，支持可配置间隔和随机时间窗口（按账号开关）
- 按账号的内联操作：同步、签到、启用/停用、编辑、删除
- 按站点的操作：置顶、归档 / 恢复、批量编辑（启用 / 停用 / 删除）、导入
- **投射渠道**：从站点账号分组自动创建本地 Octopus 渠道，支持按分组的 Key 管理、模型路由和历史追踪（在**站点渠道**页签查看）
- 按模型自动推断路由类型（openai_chat、openai_response、anthropic、gemini、volcengine、embedding）
- 支持手动添加 / 删除模型和路由类型覆盖
- 支持从 AllAPIHub 和 MetAPI 格式批量导入（上传文件或粘贴 JSON）
- 代理池集成，支持按站点、按账号和按渠道的代理选择
- 归档站点（保留账号/Key/模型，下线投射渠道），可从归档列表恢复
- **自动化**页签合并自动同步和自动签到间隔配置

---

### 🌍 代理池

可从应用外壳工具栏访问的共享代理配置池：

- 命名代理配置，支持 URL、协议（SOCKS5 / HTTP / HTTPS）、启用/禁用和备注
- 代理模式：`direct`、`system`、`pool`、`inherit`
- 针对可配置测试 URL 的代理连通性测试
- **引用树**：展示哪些站点、站点账号、托管渠道和渠道使用了每个代理
- 引用跳转导航，可深链到引用实体
- 当代理有活跃引用时，禁止删除

---

### 🔁 模型映射

全局模型名改写规则，在 relay 管线中于分组解析之前生效：

- **匹配类型**：精确、通配符（glob）和正则
- **目标模型**：改写后的模型名
- **优先级排序**：按优先级顺序评估规则
- **分组作用域**：可选仅应用于特定分组
- **启用/禁用开关**：每条规则可独立开关

---

### ☁️ WebDAV 云备份

基于 WebDAV 的自动云备份，提供完整生命周期管理：

- 可配置基础 URL、凭证、远程路径、自动备份间隔（默认 6 小时）和最大备份保留数
- 启用前连接测试
- 手动触发备份
- 远程备份文件列表（含大小信息）
- 从任意远程备份一键恢复
- 删除远程备份
- 作为设置页独立卡片提供

---

### 🔑 API 凭证档案与 CLI 导出

可复用的 API 凭证档案存储 Base URL + API Key 对，方便快速访问：

- 健康探测类型：`text_gen`、`models_list`、`tool_calling`、`structured_output`
- 按凭证健康状态追踪
- 通过 `security.encryption_key` 加密存储
- 标签和备注用于组织管理

**CLI 配置导出：**

为下列客户端工具生成即用配置片段：

| 工具 | 格式 |
|------|------|
| Claude Code | `~/.claude/settings.json` 环境变量 |
| Codex | `~/.codex/auth.json` 和 `config.toml` 环境变量 |
| Gemini CLI | 环境变量 |
| Cherry Studio | JSON 供应商导入配置 |
| Kilo Code | JSON 设置块 |

---

### 🚨 通知中心与告警

通知模块是统一中心，聚合系统事件、告警触发和套餐供应商通知，支持严重级别、已读/归档状态、过滤和 SSE 实时推送。告警规则监控系统健康并触发通知：

**告警规则类型：** 错误率（支持可配置作用域——按渠道 / 按分组 / 全局——和滑动窗口评估）、费用阈值、额度超限、渠道下线，以及号池账号异常（`pool_account_error`，按账号作用域）。

**通知渠道：**

| 渠道 | 配置项 |
|------|--------|
| Webhook | URL、方法、Header |
| Gotify | 服务地址、应用 Token |
| Email | SMTP 设置、收件人 |
| Telegram | Bot Token、Chat ID |
| 飞书 | Webhook Key |
| 钉钉 | 机器人 Access Token、可选 HMAC-SHA256 签名密钥 |
| 企业微信 | 群机器人 Key |
| ntfy | Topic URL、可选 Access Token |

每条规则的告警状态和历史均会被追踪，支持可配置的评估间隔。

**用量报表：** 支持按日 / 周 / 月调度用量报表，通过已配置的通知渠道送达，并记录报表历史。

---


## 🔌 客户端接入

### OpenAI SDK

```python
from openai import OpenAI
import os

client = OpenAI(   
    base_url="http://127.0.0.1:8080/v1",   
    api_key="sk-octopus-P48ROljwJmWBYVARjwQM8Nkiezlg7WOrXXOWDYY8TI5p9Mzg", 
)
completion = client.chat.completions.create(
    model="octopus-openai",  # 填写正确的分组名称
    messages = [
        {"role": "user", "content": "Hello"},
    ],
)
print(completion.choices[0].message.content)
```

### Claude Code

编辑 `~/.claude/settings.json`

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8080",
    "ANTHROPIC_AUTH_TOKEN": "sk-octopus-P48ROljwJmWBYVARjwQM8Nkiezlg7WOrXXOWDYY8TI5p9Mzg",
    "API_TIMEOUT_MS": "3000000",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "ANTHROPIC_MODEL": "octopus-sonnet-4-5",
    "ANTHROPIC_SMALL_FAST_MODEL": "octopus-haiku-4-5",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "octopus-sonnet-4-5",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "octopus-sonnet-4-5",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "octopus-haiku-4-5"
  }
}
```

### Codex

编辑 `~/.codex/config.toml`

```toml
model = "octopus-codex" # 填写正确的分组名称

model_provider = "octopus"

[model_providers.octopus]
name = "octopus"
base_url = "http://127.0.0.1:8080/v1"
```
编辑 `~/.codex/auth.json`

```json
{
  "OPENAI_API_KEY": "sk-octopus-P48ROljwJmWBYVARjwQM8Nkiezlg7WOrXXOWDYY8TI5p9Mzg"
}
```

### CLI 配置导出

对于其他客户端（Gemini CLI、Cherry Studio、Kilo Code），请使用管理控制台中 API 凭证档案面板内置的 **CLI 导出** 功能来生成即用配置片段。

---

## 🏗️ 架构

Octopus 采用清晰的分层 Go 架构：

```
cmd/                    # 程序入口（Cobra CLI）
internal/
├── apperror/           # 应用错误类型（机器码 + HTTP 状态码 + i18n 参数）
├── conf/               # 配置加载与构建元信息
├── client/             # 出站 HTTP 客户端池（超时分桶、代理、SSRF 防护）
├── db/                 # 数据库连接与迁移（SQLite/MySQL/PostgreSQL）
│   └── migrate/        # 版本化 Schema 迁移（编号以该目录内文件为准）
├── model/              # 领域类型（Channel、Group、APIKey、User、Site、ProxyConfiguration、ModelMapping……）
├── op/                 # 按领域拆分的业务逻辑操作
│   ├── airoute/        # AI 路由生成、进度追踪、服务池和兼容辅助逻辑
│   ├── alert/          # 告警规则评估和通知分发
│   ├── analytics/      # 仪表盘、用量、路由健康、评估和延迟查询
│   ├── apikey/         # API Key 增删改查和验证
│   ├── audit/          # 审计日志持久化
│   ├── backup/         # 数据库导出/导入、WebDAV 云备份调度器
│   ├── cacheusage/     # 缓存使用追踪
│   ├── channel/        # 渠道 CRUD、同步、分组、密钥、托管渠道投射和 Base URL 辅助逻辑
│   ├── credential/     # API 凭证档案管理（含加密）
│   ├── dbmigration/    # SQLite/MySQL/PostgreSQL 之间的实时数据库迁移
│   ├── errorlog/       # 后端错误日志持久化与保留策略
│   ├── group/          # 路由分组 CRUD、自动分组、分组项、测试与缓存查询
│   ├── llm/            # LLM 价格目录操作
│   ├── modelmapping/   # 模型映射规则管理
│   ├── modelnormalize/ # 模型名归一化规则
│   ├── navorder/       # 导航顺序和可见性持久化
│   ├── notification/   # 通知中心（消息、SSE 流、偏好）
│   ├── ops/            # Ops 仪表盘数据聚合（遥测、配额、健康）
│   ├── pool/           # 面向 relay 调度的号池管理
│   ├── ratelimitstore/ # RPM/TPM 限流状态
│   ├── relaylog/       # Relay 日志持久化（含异步刷写 Worker）
│   ├── remotesite/     # 远程 Hub 站点操作（余额、签到、公告、用量、令牌、兑换）
│   ├── report/         # 用量报表调度和送达
│   ├── setting/        # 设置增删改查和验证
│   ├── stats/          # 请求统计聚合、缓存和站点模型回填
│   ├── user/           # 用户管理和认证
│   └── webauthn/       # WebAuthn / Passkey 注册和认证
├── relay/              # 核心中转管线
│   ├── balancer/       # 负载均衡策略（轮询、随机、故障转移、加权、智能）
│   ├── condition/      # 请求条件评估
│   └── poolscheduler/  # 号池渠道的账号调度
├── server/             # HTTP 层（Gin）
│   ├── auth/           # JWT 认证与权限
│   ├── handlers/       # 路由处理器（每个资源一个文件）
│   ├── middleware/     # 鉴权、RBAC、CORS、限流、审计、安全、IP 白名单……
│   ├── resp/           # 响应信封辅助
│   └── router/         # 路由注册系统
├── task/               # 后台定时任务
├── transformer/        # 协议适配器
│   ├── inbound/        # 客户端→内部（OpenAI、Anthropic）
│   ├── outbound/       # 内部→上游（OpenAI、Anthropic、Cloudflare、Gemini、Volcengine、MiMo、Codex、Passthrough）
│   ├── rewrite/        # 请求规范化（可配置 Profile）
│   └── model/          # 共享适配器类型与接口
├── hub/                # 远程站点适配器接口、注册表、HTTP 客户端和平台专属适配器
├── planprovider/       # 上游订阅套餐监控（Codex、MiMo、StepFun、SenseNova、余额类厂商）
├── poolhealthcheck/    # 号池账号周期健康巡检
├── pooltokenrefresh/   # 号池 OAuth Token 自动刷新（含退避）
├── poolscheduledtest/  # 号池计划连通性测试与账号恢复
├── store/              # 可选缓存/状态后端（KVStore、RateLimitStore、StatsStore、RuntimeStateStore、ChannelDelayStore）：内存 + Redis
├── helper/             # 横切辅助（AI 路由、渠道/分组探测、价格、通知）
├── price/              # LLM 价格目录（models.dev 同步）
├── pkg/                # 号池账号第三方 OAuth 流程辅助（Anthropic / OpenAI / Gemini CLI / xAI）
├── update/             # 自更新机制
├── utils/              # 工具库（缓存、限流、语义缓存、分词器、加密……）
├── site/               # 薄门面，全部转发到 sitesync
└── sitesync/           # 站点同步、投射与签到实现
```

**Relay 数据流：**

```
客户端请求
    ↓
模型映射（全局名称改写）
    ↓
inbound.TransformRequest（原始格式 → 内部通用格式）
    ↓
outbound.TransformRequest（内部格式 → 上游格式）
    ↓
http.Do（转发到上游供应商）
    ↓
outbound.TransformResponse（上游响应 → 内部通用格式）
    ↓
inbound.TransformResponse（内部格式 → 客户端格式）
    ↓
客户端响应
```

流式场景中，相同的管线会通过 `TransformStream` 逐条处理 SSE 事件。

**Hub 适配器：**

Hub 远程站点管理采用适配器架构。各适配器包在 `init()` 中为对应站点类型注册专属处理器（当前清单以 `internal/hub/` 各子包与 `internal/hub/README.md` 为准），注册表对没有专属适配器的站点类型回退到 `common`（One API / New API 族）适配器：

| 适配器 | 站点类型 |
|--------|----------|
| `common` | 直接注册 `new-api`、`unknown`；`veloera`、`done-hub`、`one-hub`、`anyrouter` 经注册表回退由其处理（One API / New API 族） |
| `octopus` | `octopus`（自适应适配器） |
| `aihubmix` | `aihubmix` |
| `axonhub` | `axonhub` |
| `claudecodehub` | `claude-code-hub` |
| `sub2api` | `sub2api` |
| `sapi` | `sapi`（用户账号/密码登录，带 Token 缓存） |

`ldoh` 包提供公开站点发现功能（非适配器）。

每个适配器实现 `SiteAdapter` 接口，涵盖用户信息、签到、模型、价格、令牌、渠道、公告、状态、兑换和用量日志。

**前端（Next.js 16 App Router）：**

```
web/src/
├── api/               # API 客户端与端点 Hooks（TanStack Query）
├── app/               # Next.js App Router 页面
├── components/
│   ├── modules/       # 领域模块（渠道、分组、API Key、远程站点、站点、号池、代理池、模型映射、凭证……）
│   ├── ui/            # UI 原语（基于 Radix）
│   ├── common/        # 共享组件
│   ├── nature/        # 动画背景与特效
│   └── animate-ui/    # 动效 UI 组件
├── hooks/             # 自定义 Hooks
├── lib/               # 工具、国际化、日志、时区辅助
├── provider/          # React Context 提供者
├── route/             # 懒加载路由配置
└── stores/            # Zustand 客户端状态
```

## 🕐 时区架构

Octopus 涉及时区的三层独立概念：

| 层 | 控制方 | 影响 |
|----|---------|---------|
| **容器时区** | `ENV TZ` / `-e TZ=` | 服务端日志时间戳、`time.Now()` 返回值 |
| **统计时区** | 管理端设置 → `stats_timezone`（IANA 名，如 `Asia/Shanghai`） | 每小时/每天统计数据的日期归入 |
| **前端展示时区** | 管理端设置 → 用户偏好 | 所有页面上的时间显示格式 |

三层独立配置：容器时区影响服务端运行时，统计时区影响数据聚合，前端展示时区只影响用户看到的时间文本。

## 🔐 安全

- **JWT 认证**：管理 API 使用 JWT 令牌，支持可配置过期时间。登录频率限制可防止暴力破解（可配置窗口期和最大失败次数）。
- **角色访问控制**：服务端 RBAC，内置 `admin`、`editor`、`viewer` 三种角色，每次请求从数据库重新加载角色。
- **API Key 安全**：API Key（`sk-octopus-...`）支持模型白名单、IP/CIDR 白名单、过期时间、费用上限、RPM/TPM 限额和按模型配额。
- **静态加密**：敏感存储数据（凭证档案、站点密码等）通过 `security.encryption_key` 使用 AES-256-GCM 加密。
- **CORS 管理**：标签式 CORS 白名单管理器，支持 `*`（允许所有）、特定域名或全部拒绝（空）。
- **Viewer 域名脱敏**：Hub 相关管理数据对 viewer 账号进行域名脱敏，覆盖站点、远程站点、凭证、渠道和 URL 设置。

## 🤝 致谢

- 🙏 [looplj/axonhub](https://github.com/looplj/axonhub) - 本项目的 LLM API 适配模块直接源自该仓库的实现
- 📊 [sst/models.dev](https://github.com/sst/models.dev) - AI 模型数据库，提供模型价格数据
- 💡 [qixing-jk/all-api-hub](https://github.com/qixing-jk/all-api-hub) - Hub 概念和功能设计灵感来源
- 🛠️ [Hureru/octopus](https://github.com/Hureru/octopus) - Hub 的原始实现
- 🏊 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) - 号池账号管理的 OAuth 流程（Anthropic / OpenAI / Gemini CLI / xAI）与账号管理交互设计移植自该项目
