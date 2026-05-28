# 贡献指南

感谢您为项目做出贡献，请遵循以下规范。

## PR 范围

- 每个 PR 只允许包含一个变更主题（一个新增功能、一个 BUG 修复或一次文档同步）。
- 若存在多个变更主题，请拆分为多个独立 PR 提交。
- 若变更影响用户可见行为、发布流程、运行方式或配置项，请同步更新相关 Markdown 文档。
- 文档变更应以当前代码、配置、路由、命令和 git 历史为事实来源，不要只依据旧文档改写。

## AI 使用规范

- 允许使用 AI 辅助。
- 提交者对最终内容负责，提交前必须完成人工审查。

## 提交前检查

- [ ] 本次 PR 仅包含一个变更主题。
- [ ] AI 参与的内容已完成人工审查。
- [ ] 受影响的文档已同步更新（如 `README.md`、`README_zh.md`、`web/README.md`）。
- [ ] 若前端导航、首页结构或设置页卡片发生变化，请同步检查 `AGENTS.md`、`CLAUDE.md` 与 `web/README.md` 是否仍与实现一致。
- [ ] 若后端领域逻辑移动到 `internal/op/*` 子包，请同步检查架构说明、常用入口表和相关开发说明。
- [ ] 若涉及发布，请确认示例版本号、Docker 标签和命令说明未过期。
- [ ] 若只修改文档，请确认 `git diff -- *.md web/*.md` 覆盖全部预期变更，且没有源代码文件被误改。
- [ ] 若新增管理端接口为下载型端点（直接返回文件流 / JSON dump 而不经 `{code,message,data}` envelope），请在 handler 注释中标注"下载接口，有意不使用标准 envelope"，并在前端消费处补充"不经 apiClient 解包"说明。
- [ ] 用户可见的时间展示不得直接使用裸 `toLocaleString()` 或裸 `dayjs().format()`——必须经由 `@/lib/time` 统一工具格式化，以遵循用户设置的应用时区。

## 静态约束

- 后端基础静态约束使用 `pnpm lint:go` 做全包编译检查；严格 Go lint 使用根目录 `.golangci.yml`，可手动执行 `pnpm lint:go:strict`。
- 前端静态检查使用 `web/eslint.config.mjs`，CI 与本地统一执行 `pnpm lint:web`。
- 仓库级文本格式由根目录 `.editorconfig` 约束；Go 使用制表符，其余源码与配置默认 2 空格。
- Go 代码格式化以 `gofmt` / `goimports` 为准；前端暂不额外引入 Prettier，避免与现有 ESLint/Next 约束重叠。

## 测试约束

- 仓库统一检查入口为根目录 `pnpm check`，依次执行静态检查与测试。
- 后端测试入口为 `pnpm test:go`（即 `go test ./...`）。修改后端时，优先先跑受影响包，再决定是否全量执行。
- 前端测试入口为 `pnpm test:web`，包含 `test:i18n` 与 `test:unit`。
- 影响前端构建链路、路由、导出行为或嵌入式管理端时，应补跑 `cd web && pnpm build` 或直接执行 `pnpm check`。

## 流程约束

- GitHub PR / push 质量门由 `.github/workflows/quality.yml` 执行，覆盖 Go 格式、Go lint、Go test、前端 lint、前端测试与前端构建。
- 本地提交流程通过 Husky 触发：`pre-commit` 执行 `lint-staged`，`commit-msg` 执行 `commitlint`。
- Commit message 采用 Conventional Commits 规范，配置见 `.commitlintrc.json`。
- 首次拉取后请在仓库根目录执行 `pnpm install`，以安装提交流程所需开发依赖并启用 Git hooks。

## 建议验证

- 后端变更：优先运行相关包测试，再视范围运行 `go test ./...`。
- 前端变更：运行 `cd web && pnpm lint`，必要时再运行 `pnpm build`。
- 文档变更：检查命令、路径、版本号、Docker 标签、迁移编号和中英文 README 是否一致。

常用统一命令：

- `pnpm lint`
- `pnpm test`
- `pnpm check`