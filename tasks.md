# MEOW 工程化任务台账

本文档随开发实时续写，记录当前计划、任务状态、验收命令和风险闭环。

## Stage 状态

| Stage | 目标 | 状态 | 验收 |
|---|---|---|---|
| Stage 0 | 建立 `design.md` / `tasks.md` / 项目模型 / 基线 | Done | 文档存在，Mermaid 图和风险表完整 |
| Stage 1 | 修复高风险安全解包与 cache clear | Done | 高风险 focused 测试通过 |
| Stage 2 | 修复远程读取限流、RPM metadata、config/doctor | Done | 中风险 focused 测试通过 |
| Stage 3 | 补齐测试矩阵 | Done | Go/TUI/CLI 风险测试覆盖 |
| Stage 4 | 加强 CI 门禁 | Done | CI 包含 coverage/docs/race gates |
| Stage 5 | 复审剩余项闭环 | Done | P1/P2 回归测试补齐 |
| Stage 6 | Go TUI 主线工程化改造 | Done | Go TUI 工作流测试和 CI/文档收敛 |

## Agent 分工

| Agent | 职责 | 状态 |
|---|---|---|
| Architect Agent | 维护 `design.md`、Mermaid 模型、接口决策 | Active |
| Taskkeeper Agent | 维护 `tasks.md`、状态和验收记录 | Active |
| Security Backend Agent | 修复安全解包、cache clear、远程限流 | Done |
| Quality Agent | 补测试、覆盖率基线、评分 | Done |
| CI DevEx Agent | 改造 GitHub Actions、docs gate、coverage gate | Done |
| TUI Agent | 保护 TUI runner 与 Go JSON mode 兼容 | Done |
| Review Agent | 完成后评分和风险闭环核对 | Done |
| Go TUI Agent | 收敛 `meow tui` 到 Go Bubble Tea 主线，删除 OpenTUI 子项目 | Done |

## 当前任务列表

| ID | Stage | Task | 状态 | 验收命令/证据 |
|---|---|---|---|---|
| T0-1 | 0 | 新增 `design.md`，包含工程规则、架构图、风险表、测试矩阵 | Done | `Test-Path design.md` |
| T0-2 | 0 | 新增 `tasks.md`，记录 stage/agent/task/验收状态 | Done | `Test-Path tasks.md` |
| T1-1 | 1 | debug package 解包前校验 archive 成员路径 | Done | `go test ./internal/backend` |
| T1-2 | 1 | `vmlinux` realpath 限制在工作目录内 | Done | backend 单元测试 + 脚本检查 |
| T1-3 | 1 | `cache clear` 增加 sentinel / `--force` / 危险目录拒绝 | Done | `go test ./internal/cache ./cmd` |
| T2-1 | 2 | symbol source index 读取大小上限 | Done | `go test ./internal/symbolsources` |
| T2-2 | 2 | RPM `repomd.xml` / primary metadata 限流和流式解析 | Done | `go test ./internal/resolver` |
| T2-3 | 2 | `primary_db` 明确 unsupported | Done | resolver 单元测试 |
| T2-4 | 2 | build/parse/cache/verify 使用 config 默认值，flag 覆盖 | Done | `go test ./cmd` |
| T2-5 | 2 | doctor 增加 `vol` 检查并区分 build/verify 能力 | Done | backend/cmd 测试 |
| T3-1 | 3 | 补齐安全、远程、config、JSON mode 测试 | Done | `go test ./... -count=1` |
| T3-2 | 3 | 补齐历史 OpenTUI runner 测试 | Retired | Stage 6 删除 OpenTUI 子项目后由 Go TUI 测试替代 |
| T4-1 | 4 | 增加 coverage baseline gate | Done | CI workflow + script |
| T4-2 | 4 | 增加 docs gate | Done | CI workflow + script |
| T4-3 | 4 | 增加 Go race gate | Done | CI workflow |
| T5-1 | 5 | `cache clear --force` 拒绝当前工作目录祖先目录 | Done | `go test ./internal/cache -run TestClearRefusesCurrentWorkingDirectoryAncestorEvenWithForce -count=1 -v` |
| T5-2 | 5 | 安全解包 shell 函数增加真实执行测试 | Done | `go test ./internal/backend -run TestSafeArchiveShellFunctionExecutesValidation -count=1 -v` |
| T6-1 | 6 | 删除 Bun/OpenTUI 子项目和 `tuitest` 诊断命令 | Done | `git ls-files tui cmd/tuitest.go` 不再列出保留文件 |
| T6-2 | 6 | `meow tui` 增加 Options/config defaults/CLI flag 覆盖 | Done | `go test ./cmd -run TestApplyTUIConfigDefaults -count=1` |
| T6-3 | 6 | Go TUI 增加命令解析、input mode、argv 参数构建和 cache clear 确认保护 | Done | `go test ./internal/tui -run TestParseFields -count=1` |
| T6-4 | 6 | Go TUI runner 支持 stdout/stderr 流式日志、取消、非零退出 | Done | `go test ./internal/tui -run TestExecRunner -count=1` |
| T6-5 | 6 | Go TUI 实现 doctor/preflight/build/verify/run/workflow/cache 工作流 | Done | `go test ./internal/tui -run TestWorkflow -count=1` |
| T6-6 | 6 | CI 移除 frontend job，Go TUI 纳入 backend 测试 | Done | `.github/workflows/ci.yml` backend `go test ./...` |

## 验收记录

- Stage 0: `design.md` / `tasks.md` 已创建。
- Stage 1/2 focused tests: `go test ./cmd ./internal/cache ./internal/backend ./internal/resolver ./internal/symbolsources -count=1 -p 1` 通过。
- Full Go: `GOTMPDIR=.gotmp go test ./... -count=1 -p 1` 通过。
- Race: `GOTMPDIR=.gotmp go test -race ./cmd ./internal/...` 通过。
- Coverage focused: total 42.5%，高于 CI baseline 35.0%。
- TUI: `cd tui && ..\node_modules\.bin\bun.cmd test` 通过，6 pass。
- Windows local note: `build --dry-run` 按设计在非 Linux 主机返回 Linux-only 错误；CI Ubuntu smoke 覆盖该路径。
- Review closeout: `go test ./internal/cache -run TestClearRefusesCurrentWorkingDirectoryAncestorEvenWithForce -count=1 -v` 通过。
- Review closeout: `go test ./internal/backend -run TestSafeArchiveShellFunctionExecutesValidation -count=1 -v` 在 Windows 按 Linux-only 条件 skip；该测试会在 Linux CI 实际执行 shell 校验。
- Windows local note: `go test ./internal/backend -count=1 -p 1` 包内测试输出 `ok`，但本地 Go 清理 `.gotmp` 测试二进制时遇到 Windows 文件锁导致命令退出码为 1。
- Stage 6 focused: `go test ./internal/tui ./cmd -count=1` 包内测试输出 `ok`；本地 Windows 偶发测试二进制清理锁可能导致命令退出码为 1。

## PR 评分记录

| Stage | Score | 说明 |
|---|---|---|
| Stage 0 | 90 | 文档、架构图、风险表和任务台账已落地 |
| Stage 1 | 90 | 高风险已修复并有 focused 测试 |
| Stage 2 | 90 | 中风险已修复，focused 与全量测试已验证 |
| Stage 3 | 90 | Go/TUI/race/focused coverage 均已验证 |
| Stage 4 | 90 | CI 增加 docs、coverage baseline、race gates |
| Stage 5 | 92 | 复审 P1/P2 已闭环，新增针对性回归测试 |
| Stage 6 | 90 | Go TUI 主线收敛，OpenTUI 已移除，符号生成工作流和回归测试已补齐 |

## Stage 6 TUI Command Surface Addendum

| ID | Stage | Task | Status | Evidence |
|---|---|---|---|---|
| T6-7 | 6 | Productize TUI command surface with explicit `/mode`/`/source`, `/unset`, `/reset`, aliases, and plugin startup presets | Done | `go test ./internal/tui ./cmd -run "Test(TUI|ApplyTUI|ModeUnset|ExplicitMode|ValidateBuildInput)" -count=1` |
| T6-8 | 6 | Harden short-height and narrow-width TUI rendering with compact log-first layout, clipped log lines, and scroll regression tests | Done | `go test ./internal/tui -run "TestView|TestLogScroll" -count=1 -v` |

### 2026-06-09 Validation Note

- Focused TUI/cmd tests passed at package level; local Windows may still report Go test binary cleanup locks after `ok`.
- Short panel regression tests now cover 4/8/12/13/18 row terminals, 8/20/39/60 column edge cases, long-line clipping, and log history scrolling.
