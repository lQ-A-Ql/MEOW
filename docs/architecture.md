# meow 架构设计文档

## 1. 系统全景

meow 是一个 Linux 内核 Volatility 3 符号表生成工具，由两个独立子系统组成：

```
┌─────────────────────────────────────────────────────────────┐
│                 Go TUI (cmd/tui.go + internal/tui)          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐  │
│  │ slash 命令 │  │ 状态模型  │  │ 三栏渲染  │  │ Runner argv │  │
│  │commands.go│  │ app.go   │  │ app.go   │  │ runner.go   │  │
│  └──────────┘  └──────────┘  └──────────┘  └─────┬──────┘  │
│                                                   │         │
└───────────────────────────────────────────────────┼─────────┘
                                                    │ argv 数组
                    ┌───────────────────────────────┼─────────┐
                    │          Go CLI 后端           │         │
                    │  ┌──────┐  ┌──────┐  ┌──────┐ │         │
                    │  │parse │  │build │  │verify│ │         │
                    │  └──┬───┘  └──┬───┘  └──┬───┘ │         │
                    │     │         │         │      │         │
                    │  ┌──┴─────────┴─────────┴───┐  │         │
                    │  │       internal/           │  │         │
                    │  │ banner → symbolsources    │  │         │
                    │  │        ↓                  │  │         │
                    │  │    resolver → downloader   │  │         │
                    │  │        ↓                  │  │         │
                    │  │   backend (native bash)   │←─┘         │
                    │  │        ↓                  │            │
                    │  │   volatility (vol CLI)    │            │
                    │  └───────────────────────────┘            │
                    └───────────────────────────────────────────┘
                                                    │
                              ┌─────────────────────┼──────────┐
                              │   外部工具链 (Linux 原生)        │
                              │  bash, dpkg-deb, tar, xz,      │
                              │  rpm2cpio, cpio, dwarf2json,    │
                              │  vol (Volatility 3)              │
                              └─────────────────────────────────┘
```

### 模块职责

| 层 | 路径 | 职责 |
|----|------|------|
| CLI 入口 | `main.go` | `--json` 预解析、调用 `cmd.Execute()` |
| 命令注册 | `cmd/root.go` | 全局 flag 解析、子命令分发 |
| 命令实现 | `cmd/*.go` | 各子命令的 flag 定义、输入收集、进度渲染 |
| Banner 解析 | `internal/banner/` | 从 banner 字符串提取 distro/kernel/arch/pkgver |
| 远程符号源 | `internal/symbolsources/` | TXT 配置 + 内置 Abyss-W4tcher 源，精确 banner 匹配 |
| 包候选生成 | `internal/resolver/` | 生成 .ddeb/.deb/.rpm 候选 URL，HTTP 探测 |
| 下载器 | `internal/downloader/` | HTTP 下载 + 进度回调 + 断点(.part) |
| 缓存 | `internal/cache/` | URL hash 键、元数据 JSON、`--force` 覆盖 |
| 后端执行 | `internal/backend/` | `//go:embed` 嵌入 bash 脚本，Linux 原生执行 |
| Volatility 封装 | `internal/volatility/` | 调用 `vol` CLI 提取 banner、验证符号 |
| 符号文件命名 | `internal/symbols/` | `FileName()` 统一输出文件名格式 |
| Logo | `internal/logo/` | ASCII art 渐变 logo |
| TUI 入口 | `cmd/tui.go` | 注册 `meow tui` flags，合并 config 默认值，启动 Bubble Tea |
| TUI 应用 | `internal/tui/app.go` | Bubble Tea model、键盘、三栏/双栏/单栏响应式布局 |
| TUI 命令 | `internal/tui/commands.go` | `/` 前缀命令解析、带引号参数、cache clear 确认保护 |
| TUI 运行器 | `internal/tui/runner.go` | argv 子进程执行，stdout/stderr 流式回调，取消/退出码处理 |
| TUI 工作流 | `internal/tui/workflow.go` | doctor → preflight → build → verify、cache、plugin run |

---

## 2. Go 后端架构

### 2.1 CLI 层

```
main.go
  └─ cmd.Execute()          // cmd/root.go
       ├─ 全局 flag: --json, --verbose, --help
       └─ 子命令分发 (flag.NewFlagSet)
            ├─ parse         // cmd/parse.go
            ├─ build         // cmd/build.go
            ├─ verify        // cmd/verify.go
            ├─ doctor        // cmd/doctor.go
            ├─ cache         // cmd/cache.go
            └─ config        // cmd/config.go
```

每个子命令在 `init()` 中通过 `cmd.Register()` 注册。全局 `--json` 在 `main.go` 预解析以决定是否抑制 logo，子命令内再做局部 `--json` 解析。

### 2.2 Build 数据流水线

```
输入源
  ├─ --banner-file / stdin banner
  ├─ --mem (内存镜像 → vol banners.Banners)
  ├─ --kernel + --pkgver (手工)
  ├─ --debug-package / --debug-package-url (本地/远程包)
  └─ --vmlinux (直接 vmlinux)
       │
       ▼
  banner.ParseBanner()           ← internal/banner/
  提取: distro, kernel, arch, pkgver
       │
       ▼
  symbolsources.Match()          ← internal/symbolsources/
  精确匹配远程 ISF 索引
       │
       ├─ 命中 → 下载 .json.xz → 完成
       │
       └─ 未命中
            │
            ▼
       resolver.GenerateCandidates()  ← internal/resolver/
       生成包候选 URL 列表
            │
            ├─ --repo-url → 读取 repomd.xml → primary.xml.gz 精确匹配
            │
            └─ HTTP 探测 (HEAD → GET Range fallback)
                 │
                 ▼
            downloader.Download()     ← internal/downloader/
            带进度回调的 HTTP 下载
                 │
                 ▼
            cache.Store()             ← internal/cache/
            URL hash 键 + 元数据 JSON
                 │
                 ▼
            backend.BuildFrom*()      ← internal/backend/
            bash --noprofile --norc -c 执行嵌入脚本
                 │
                 ├─ 解压 .ddeb/.deb/.rpm
                 ├─ 查找/解压 vmlinux
                 ├─ dwarf2json linux --elf
                 ├─ xz 压缩
                 └─ 输出 ${Distro}_${Kernel}_${PkgVer}_${Arch}.json.xz
```

### 2.3 Banner 解析算法

`internal/banner/parse.go` 中的 `ParseBanner()` 按优先级尝试：

1. **Ubuntu** — 匹配 `Ubuntu` 关键词，提取 `x.yy.z-NN` 格式内核版本，查询 `ddebs.ubuntu.com` 获取精确包版本
2. **Debian** — 匹配 `Debian` 关键词，从 `#... Debian x.yy.z-NN` 提取内核包版本（排除 GCC 版本 `Debian 10.2.1-6`）
3. **RHEL/CentOS** — 匹配 `Red Hat` / `CentOS` / `Rocky` / `Alma` / `Fedora`
4. **Amazon Linux** — 匹配 `Amazon Linux`
5. **SUSE** — 匹配 `SUSE` / `openSUSE`

每种 distro 的解析器提取 `KernelInfo`:
- `Distro` — 发行版标识
- `Kernel` — 内核版本字符串
- `Arch` — 架构 (amd64/arm64)
- `PackageVersion` — 包版本（Ubuntu/Debian 自动，RPM 系为空需手工或 repo 查询）

### 2.4 Resolver 探测策略

```
对每个候选 URL:
  1. HEAD 请求
     ├─ 200 → 存在，返回 URL
     ├─ 403/405/501 → 服务器拒绝 HEAD
     │    └─ GET Range: bytes=0-0
     │         ├─ 200/206 → 存在
     │         └─ 404 → 不存在
     └─ 404 → 不存在
```

`ProbeFunc` 回调报告当前序号/总数/URL，供进度显示。`--probe-timeout` 控制总探测超时。

### 2.5 后端脚本与 Marker 协议

`internal/backend/scripts/` 中的 bash 脚本通过 stdout 输出结构化 marker：

| Marker | 含义 |
|--------|------|
| `VOLSYM_STAGE=extract` | 开始解压 |
| `VOLSYM_EXTRACT_TOTAL=N` | 待解压文件总数 |
| `VOLSYM_EXTRACT_FILE=i/N:path` | 第 i 个文件解压完成 |
| `VOLSYM_STAGE=find_vmlinux` | 开始查找 vmlinux |
| `VOLSYM_VMLINUX=path` | 找到的 vmlinux 路径 |
| `VOLSYM_STAGE=dwarf2json` | 开始运行 dwarf2json |
| `VOLSYM_STAGE=compress` | 开始 xz 压缩 |
| `VOLSYM_STAGE=move` | 移动输出文件 |
| `VOLSYM_SYMBOL=path` | 最终符号文件路径 |
| `VOLSYM_STAGE=done` | 完成 |

`cmd/progress.go` 解析这些 marker 渲染双进度条（整体构建进度 + 解压文件进度）。JSON 模式不启用进度输出。

**错误处理**：脚本使用临时文件捕获解压输出，检查提取命令退出码。提取失败时立即报错退出，不会被 `while read` 子 shell 吞没退出码。

### 2.6 包格式识别

`internal/backend/packageFormatFromPath()` 按扩展名识别：

| 扩展名 | 格式 | 解压方式 |
|--------|------|----------|
| `.rpm` | rpm | `rpm2cpio \| cpio -idmv` |
| `.deb` | deb | `dpkg-deb --fsys-tarfile \| tar -xvf` |
| `.ddeb` | ddeb | 同 deb |

vmlinux 搜索路径（按优先级）：
1. `/usr/lib/debug/boot/vmlinux-$KERNEL`
2. `/usr/lib/debug/lib/modules/$KERNEL/vmlinux`
3. `/usr/lib/debug/lib64/modules/$KERNEL/vmlinux`
4. 递归查找 `vmlinux*`、`vmlinux*.gz`、`vmlinux*.xz`、`vmlinux*.zst`

---

## 3. TUI 前端架构

### 3.1 技术栈

- **运行时**: Go CLI 内置子命令 `meow tui`
- **UI 框架**: Bubble Tea + Bubbles textinput + Lip Gloss
- **测试**: Go 单元测试和 stub runner 集成测试

### 3.2 UI 布局

```
>=100 cols:  [Input/Environment] [Workflow/Logs] [Plugin/Results]
70-99 cols: [Input/Environment] [Workflow/Logs]
<70 cols:   [Workflow/Logs]

Bottom: command bar, i focus input, r run, x cancel, q quit
```

TUI 使用压缩标题，状态符统一为 ASCII：`[OK]`、`[ERR]`、`[WARN]`、`[RUN]`，降低终端宽度差异导致的错位风险。

### 3.3 状态管理

`internal/tui.Model` 是唯一运行状态，包含路径配置、input mode、running action、cancel function、日志和最近结果。最近结果只保存 CLI JSON 的最小字段：`DoctorCheck`、`BuildSummary`、`VerifySummary`、`CacheEntry`。

### 3.4 命令系统

用户输入以 `/` 开头，参数解析支持单/双引号路径，未闭合引号直接报错。核心命令：

```
/mem <path> | /banner-file <path> | /debug-package <path>
/debug-package-url <url> | /repo-url <url> | /vmlinux <path>
/manual --distro <name> --kernel <release> --pkgver <version> [--arch <arch>]
/symbol <path> | /out <dir> | /cache-dir <dir> | /symbol-sources <path>
/vol <path> | /meow <path> | /plugin <name> | /plugin-args [args...]
/remote on|off | /force on|off
/doctor | /preflight | /build | /verify | /run | /workflow
/cache list | /cache clear --confirm [--force]
/clear | /help
```

`/cache clear` 必须显式带 `--confirm`，需要强制清理时再加 `--force`。

### 3.5 Runner 与工作流

Runner 只通过 argv 数组执行外部命令，不拼 shell 字符串。stdout/stderr 会实时追加到 TUI 日志，命令完成后返回 `CommandResult{Code, Stdout, Stderr, Duration}`。

工作流：

1. `doctor`: `meow --json doctor`
2. `preflight`: `meow --json build --dry-run ...`
3. `build`: `meow --json build ...`
4. `verify`: `meow --json verify --mem ... --symbols ... --vol ...`，无 mem 时跳过并记录 warning
5. `run`: `vol -f mem -s symbols plugin pluginArgs...`

build 成功后，如果 `symbol_path` 位于 `symbols/linux/*.json.xz`，TUI 自动把 `symbolsPath` 设置为其父级 `symbols`。

---

## 4. 关键设计决策

### 4.1 TUI 不复制 Go 逻辑

TUI 仅负责状态收集、子进程调用和结果渲染。所有符号生成逻辑保留在 Go CLI 中，TUI 通过 Go `exec.CommandContext(command, args...)` 调用 `meow` 和 `vol`，避免逻辑重复和维护负担。

### 4.2 argv 数组而非 shell 字符串

TUI Runner 始终使用 argv 数组执行外部命令，禁止把用户输入拼接为 shell 命令字符串。这消除了 shell 注入风险，且路径中的空格不会被误解析。

### 4.3 Go TUI 主线

`meow tui` 是唯一正式 TUI 入口。历史 Bun/OpenTUI 子项目已删除，避免 CLI 入口、文档与 CI 门禁分裂。

### 4.4 提取错误处理（临时文件模式）

bash 脚本中 `cmd | while read` 管道的问题：`while` 循环的退出码总是 0，即使 `cmd` 失败。解决方案是先将 `cmd` 输出写入临时文件、检查退出码、再从文件逐行读取。

### 4.5 HTTP HEAD → GET Range 探测

某些 CDN/镜像服务器拒绝 HEAD 请求（返回 403/405/501）。Resolver 在 HEAD 失败时 fallback 到 `GET Range: bytes=0-0`，只获取第一个字节来确认资源存在。

### 4.6 远程 ISF 优先

`build` 流程首先检查远程符号源索引（`symbol-sources.txt` 中的 ISF 仓库）。如果 banner 精确匹配，直接下载 `.json.xz`，跳过整个 debug package 下载→解压→dwarf2json 链路。这大幅缩短常见发行版的符号获取时间。

---

## 5. 测试策略

| 层 | 工具 | 覆盖范围 |
|----|------|----------|
| Go 单元测试 | `go test ./...` | banner 解析、resolver 候选/probe、cache 元数据、backend marker 解析、ShellQuote、命令 flag |
| Go 冒烟测试 | CI smoke | parse dry-run、build dry-run (Ubuntu + RPM) |
| Go TUI 单元测试 | `go test ./internal/tui ./cmd` | options、命令解析、argv 参数构建、runner、workflow、view |
| CI 门禁 | GitHub Actions | docs gate + backend job: lint、unit、coverage、race、build、smoke |
