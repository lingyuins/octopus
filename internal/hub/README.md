# Hub — 远程站点管理

Hub 提供远程站点账户、余额、签到、模型与 Token 管理。前端入口是 Hub，账号级操作集中在站点卡片中。

## 代码入口

| 入口 | 职责 |
| --- | --- |
| `adapter.go` | `SiteAdapter` 接口、请求与响应类型 |
| `registry.go` | 按 `site_type` 注册与获取适配器 |
| `httpclient.go` | 共享 HTTP 客户端与 `FetchJSON` |
| `common/` | One API / New API 默认适配器与兜底 |
| `aihubmix/`、`axonhub/`、`claudecodehub/`、`octopus/`、`sapi/`、`sub2api/` | 各站点专用适配器 |
| `ldoh/` | 公开站点发现；不注册 `SiteAdapter` |
| `../op/remotesite/` | 远程站点 CRUD、刷新、余额、签到、公告、兑换、Token、用量与渠道迁移 |
| `../op/credential/` | API 凭据档案 |
| `../sitesync/` | 多账号站点同步与投射渠道；`../site/` 为薄门面 |
| `../server/handlers/` | 路由、认证与权限控制 |

## 站点类型与适配器

已知站点类型以 `internal/model/remote_site.go` 的 `AllSiteTypes()` 为准；是否有专用实现以适配器的 `hub.Register` 为准。

| 站点类型 | 适配器 |
| --- | --- |
| `new-api`、`unknown` | `common` |
| `veloera`、`done-hub`、`one-hub`、`anyrouter` | 未注册专用实现，回退到 `common` |
| `aihubmix`、`axonhub`、`octopus`、`sapi`、`sub2api` | 同名子包 |
| `claude-code-hub` | `claudecodehub` |

`hub.Get()` 在找不到专用适配器时回退到 `new-api`。回退并不保证上游兼容所有能力；非标准站点应提供专用实现。

## SiteAdapter 接口

接口定义以 `adapter.go` 为准，主要能力包括：

- 账户与状态：`FetchUserInfo`、`FetchSiteStatus`、`FetchAnnouncement`。
- 签到与兑换：`PerformCheckIn`、`FetchCheckInStatus`、`RedeemCode`。
- 模型与价格：`FetchModels`、`FetchModelPricing`。
- Token：`FetchTokens`、`CreateToken`。
- 渠道：`ListChannels`、`CreateChannel`、`UpdateChannel`、`DeleteChannel`。
- 用量：`FetchUsageLogs`。

能力支持与不支持时的返回约定应遵循接口注释，不要仅凭站点类型判断。

## 管理 API

这些路由位于 `/api/v1/` 下，需要 JWT 认证。完整路径、HTTP 方法和权限以 `internal/server/handlers/` 内的注册代码为准，下面给出查找入口，避免重复维护端点清单。

| 路由前缀 | Handler 文件 | 权限说明 |
| --- | --- | --- |
| `/remote-site` | `remote_site.go` | 读取 `sites:read`，写入 `sites:write` |
| `/site-discovery` | `remote_site.go` | 公开目录查询仍要求 `sites:write` |
| `/balance-history` | `balance_history.go` | 读取与预测 `sites:read`，捕获 `sites:write` |
| `/checkin` | `checkin.go` | 状态与历史 `sites:read`，执行 `sites:write` |
| `/redemption` | `redemption.go` | 历史 `sites:read`，兑换 `sites:write` |
| `/usage-history` | `usage_history.go` | 查询 `sites:read`，同步 `sites:write` |
| `/announcement` | `announcement.go` | 读取 `sites:read`，刷新 `sites:write` |
| `/remote-site-token` | `remote_site_token.go` | 列表 `sites:read`，同步与导出 `sites:write` |
| `/channel-migration` | `channel_migration.go` | `sites:write` |
| `/api-credential` | `api_credential.go` | 读取 `api_keys:read`，写入 `api_keys:write` |
| `/verification` | `verification.go` | 所有接口（含探针列表）要求 `api_keys:write` |
| `/cli-export` | `cli_export.go` | `api_keys:read` |

远程 Token 的旧管理 UI 已移除，后端仍保留列表、同步、导入渠道和导出接口。不要把保留接口等同于仍有独立前端面板。

## 定时任务与自动化

`internal/task/init.go` 注册远程站点余额捕获、自动签到、公告拉取与用量同步任务。任务名和默认间隔以注册代码为准。

Hub 的“自动化”标签页复用 `SettingSiteAutomation`，用于多账号站点的自动同步与签到配置；这与上述 `hub_*` 远程站点任务不是同一组调度。

## 凭据与备份

- 站点凭据与 API Key 在库内使用 AES-256-GCM 加密，实现位于 `internal/utils/crypto/`，密文前缀为 `enc:`；无前缀值保留明文兼容。
- 密钥使用 `security.encryption_key`（环境变量 `OCTOPUS_SECURITY_ENCRYPTION_KEY`），未配置时可由持久化 `auth.jwt_secret` 派生。两者都未配置时拒绝启动，避免重启后凭据不可解密。
- 数据库备份由 `internal/op/backup/` 管理。当前 Hub 导出内容包括远程站点、余额快照、签到记录、API 凭据档案、公告和远程 Token；远程用量记录未纳入导出。完整范围以 `ExportAll` / `ImportWithModeToDB` 为准。
- **备份格式 v2 将敏感字段导出为明文，导入时用目标实例密钥重新加密。备份文件必须作为凭据文件保密存放。**
- Hub 数据增量导入跳过已存在记录；完整导入先删除既有数据再插入，应先备份并确认范围。

## 前端入口

Hub 容器位于 `web/src/components/modules/remote-site/index.tsx`，标签顺序与可见性由 `navbar/sub-tab-store.ts` 管理。

| 标签 | 实现 |
| --- | --- |
| 站点 | `modules/site/`；多账号卡片、同步、签到、归档与批量操作 |
| 站点渠道 | `modules/site-channel/`；远程投射渠道管理 |
| 自动化 | `modules/setting/SiteAutomation.tsx` |
| 额度 | `modules/plan-provider/` 的 `BalanceSection` |
| TokenPlan | `modules/plan-provider/` 的 `TokenPlanSection` |

`modules/remote-site/` 保留部分历史面板文件，但不代表当前 Hub 会渲染它们；请以容器导入与渲染代码为准。

## 添加站点适配器

1. 在 `internal/model/remote_site.go` 定义站点类型，并加入 `AllSiteTypes()`。
2. 创建 `internal/hub/<sitetype>/adapter.go`，实现 `hub.SiteAdapter`。
3. 在适配器的 `init()` 中调用 `hub.Register(model.SiteTypeXXX, &Adapter{})`。
4. **在调用方添加匿名导入**：核对 `internal/op/remotesite/remotesite.go` 与 `internal/server/handlers/remote_site.go` 的导入链。仅编写 `init()` 不会加载子包，遗漏导入会静默回退到 `common`。
5. 同步前端 `web/src/api/endpoints/remote-site.ts` 的 `SITE_TYPES` 与相关三语文案。
6. 添加适配器测试，并验证经真实导入链调用 `hub.Get()` 能取得该适配器。

New API 兼容站点通常可使用 `common`；自定义签到端点、认证或响应格式等非标准行为才需要专用适配器。

## 验证

从仓库根目录执行：

```bash
go test ./internal/hub/... ./internal/op/remotesite/...
go test ./internal/op/backup/... ./internal/utils/crypto/...
pnpm --dir web check
```

新检出且尚无前端产物时，Go 全包编译前先执行 `mkdir -p static/out && touch static/out/.keep`。
