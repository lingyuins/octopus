# 贡献指南

感谢您为项目做出贡献。提交者对最终内容负责，使用 AI 辅助的内容也必须完成人工审查。

## PR 与文档范围

- 每个 PR 只包含一个变更主题：一个功能、一个修复或一次文档同步。多个主题请拆分。
- 用户可见行为、运行方式、配置或发布流程变化时，同步相关 `README.md`、`README_zh.md`、`web/README.md` 与中英文 `wiki/`。
- 文档以当前代码、配置、路由、命令及 Git 历史为事实来源，不要仅依据旧文档改写。
- 导航、设置入口或后端目录变化时，检查对应模块表与架构说明。不要写死模块数量或迁移编号区间，应指向定义文件或目录。
- 发布时核对 `internal/conf/version.go`、`web/package.json`、`docker-compose.yml` 与 `CHANGELOG.md`，不要假设它们自动同步。
- `AGENTS.md` 等本地代理说明不入版本控制；对贡献者生效的要求应写在本指南或 `web/README.md`。

## 开发约束

- 文本格式遵循 `.editorconfig`：UTF-8、LF；Go 用制表符，其余源码与配置默认 2 空格。
- Go 格式化使用 `gofmt` / `goimports`；前端遵循 `web/eslint.config.mjs`，暂不额外引入 Prettier。
- 新增前端测试命名为 `*.test.ts`，并手动加入 `web/package.json` 的 `test:unit` 列表；该脚本不自动发现测试。
- 新增或修改用户可见文案时，同步 `web/public/locale/` 的三份语言文件。
- 用户可见时间通过 `@/lib/time` 格式化，遵循用户设置的时区；不要裸用日期的 `toLocaleString()` 或 `dayjs().format()`。数字千分位格式化不受此限制。
- 新增管理写接口必须加入 `internal/server/middleware/audit.go` 的审计白名单，或在 `internal/server/audit_route_test.go` 中写明豁免理由。
- 下载接口若直接返回文件流或 JSON dump，应在 handler 注释中标注“下载接口，有意不使用标准 envelope”，并在前端消费处说明不经 `apiClient` 解包。

## 本地验证

Go 编译依赖 `static/out/`。新检出且尚未构建前端时，先准备占位文件：

```bash
mkdir -p static/out && touch static/out/.keep
```

以下命令从仓库根目录执行：

| 命令 | 范围 |
| --- | --- |
| `pnpm lint` | Go 全包编译检查 + 前端 ESLint，不包含 Go 格式或严格 lint |
| `pnpm lint:go:strict` | 按 `.golangci.yml` 执行严格 Go lint，首次运行需要联网 |
| `pnpm test` | Go 测试 + 前端 i18n 和单元测试 |
| `pnpm check` | 上述常规 lint 与测试，**不包含前端构建** |
| `pnpm --dir web check` | 前端 lint、i18n、单元测试及静态构建 |

修改后端时先运行受影响包，例如 `go test ./internal/op/group/...`，再按范围运行全量检查。前端变更提交前执行 `pnpm --dir web check`。仅修改文档时，核对命令、路径、链接和中英文一致性，并用 `git diff -- '*.md'` 检查完整差异。

## CI 与提交

- `.github/workflows/quality.yml` 是 CI 范围的权威来源：后端执行发布脚本回归测试与 `go test ./...`；前端执行 ESLint、i18n、单元测试和静态构建。CI 不执行 Go 格式检查或 `golangci-lint`。
- 根目录 `pnpm install` 安装提交辅助工具，但仓库没有版本化的 Husky 钩子脚本。不要假设 `pre-commit` 或 `commit-msg` 会自动执行，提交前自行完成检查。
- Commit message 使用 Conventional Commits，配置见 `.commitlintrc.json`。

## 提交前清单

- [ ] PR 只有一个主题，所有改动（含 AI 辅助内容）均已人工审查。
- [ ] 已完成与变更范围匹配的验证，失败或未执行的检查已在 PR 中说明。
- [ ] 用户文档、中英文说明、前后端 DTO 和三语文案已按需同步。
- [ ] 未夹带临时脚本、生成文件修改或无关工作区改动。
