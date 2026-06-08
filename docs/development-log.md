# meow Development Log

## 2026-06-08  TUI 主线收敛

### Go TUI 工程化改造

- 决策：`meow tui` 以 Go Bubble Tea 为唯一正式 TUI 主线，第一版聚焦 Linux 符号生成闭环。
- 删除历史 Bun/OpenTUI 子项目 `tui/`，相关历史记录保留在本文档中，不再作为当前架构或 CI 主线。
- 删除 `cmd/tuitest.go` 诊断命令，避免正式 CLI 暴露测试入口。
- `cmd/tui.go` 新增 TUI flags，并合并 config 默认值；`--meow` 默认使用当前可执行文件。
- `internal/tui` 新增 Options、slash command parser、argv Runner、JSON 最小类型、doctor/preflight/build/verify/run/workflow/cache 工作流。
- TUI runner 只用 argv 调用 `meow` 和 `vol`，stdout/stderr 流式写入日志，支持取消和非零退出码。
- CI 移除 Bun frontend job，Go TUI 测试纳入 backend `go test ./...`。

### 验证

- `go test ./internal/tui ./cmd -count=1`：包内测试输出 `ok`；本地 Windows 可能因测试二进制清理锁导致命令退出码为 1。

## 2026-05-05 20:50 CST - Codex

### 文档评审

- 本轮开始时仓库没有 `docs` 目录，因此无新增开发文档可读。
- 已阅读 `PRD.md`。当前项目仍处于 PRD Milestone 1 早期，核心 MVP 链路 `build -> download/cache -> WSL -> dwarf2json -> json.xz -> verify` 尚未实现。
- 本轮先处理评审中已确认的基础缺陷，避免后续 `build` 开发建立在错误 CLI 与 resolver 语义上。

### 当前改动

- 修复 `parse --json` 未生效问题：局部 `--json` 与全局 `--json` 都能输出纯 JSON。
- 实现 root-level `--json` 与 `--verbose` 预解析；`volsym --json parse ...` 可用。
- `parse` 普通输出现在列出全部 ddeb 候选 URL。
- 放宽 Ubuntu kernel package version 解析，并避免把 GCC 的 Ubuntu 包版本误当成 kernel 包版本。
- resolver 新增 HTTP 探测接口：先 `HEAD`，遇到 `403/405/501` 时 fallback 到 `GET Range: bytes=0-0`。
- resolver 探测保留网络错误原因，同时对调用方暴露 `ErrPackageNotFound`。

### 测试

- `go test ./...`
- `go run . parse --banner-file testdata\banners\ubuntu_5.4.0_163.txt --json`
- `go run . --json parse --banner-file testdata\banners\ubuntu_5.4.0_163.txt`
- `go run . parse --banner-file testdata\banners\ubuntu_5.4.0_163.txt`

### 后续风险

- 仍未实现 `build`、`doctor`、`verify`、下载缓存、WSL 后端、符号生成。
- 当前 resolver 探测接口尚未接入 CLI；`parse` 仍只生成候选，不做网络检查。
- `go.mod` 使用 `go 1.25.0`，当前本机为 `go1.26.0 windows/amd64`，后续分发前需要确认目标用户 Go/toolchain 兼容策略。

## 2026-05-05 21:35 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与 `PRD.md`。
- 上轮后续风险中提到的 `build`、`doctor`、`verify`、下载缓存、WSL 后端、符号生成已进入实现。
- `PRD.md` 仍是当前产品源文档；本轮新增 `README.md` 补齐用户侧说明。

### 当前改动

- 新增 `build` 命令：支持 `--banner`、`--banner-file`、`--mem`、手工 `--kernel/--pkgver`、`--ddeb-url`、本地 `--ddeb`、本地 `--vmlinux`、`--dry-run`、`--force`、`--json`。
- 新增 WSL 后端：Windows 路径转 `/mnt/<drive>/...`、shell 参数引用、ddeb 解包、vmlinux 查找、`dwarf2json`、`xz` 压缩。
- 新增下载缓存：URL sha256 key、原始文件名保留、下载元数据 JSON、缓存命中复用。
- 新增 `doctor` 命令：检查 WSL、WSL distro、`bash`、`dpkg-deb`、`xz`、`curl`、`dwarf2json`，并清洗 WSL 输出中的 NUL/噪声。
- 新增 `verify` 命令：调用 Volatility 3 的 `linux.banners.Banners` 与 `linux.pslist.PsList`。
- 新增 `cache` 命令：`path`、`list`、`clear`，JSON 空列表输出为 `[]`。
- 新增 `config` 命令：`show`、`path`、`init`，默认配置路径为 `%USERPROFILE%\.volsym\config.json`。
- 新增 `README.md`：支持范围、WSL 依赖、快速开始、banner/memdump/ddeb/vmlinux 用法、缓存、配置、常见错误、Volatility 3 使用方式。
- 补充单元测试：WSL path/quote、cache metadata、downloader cache hit、symbol filename/inference。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe --json parse --banner-file testdata\banners\ubuntu_5.4.0_163.txt`
- `.\volsym.exe --json build --banner-file testdata\banners\ubuntu_5.4.0_163.txt --dry-run`
- `.\volsym.exe cache list --json`
- `.\volsym.exe config show --json`
- `.\volsym.exe doctor --json`

### 后续风险

- 未执行真实 ddeb 下载和完整 `dwarf2json` 端到端构建，原因是 Ubuntu ddeb 体积大，默认不在开发验收中拉取大包。
- WSL 当前环境检查通过，但默认 distro 输出包含多发行版；真实构建时可用 `--wsl-distro Ubuntu` 明确指定。
- `verify` 依赖本机 Volatility 3 与真实 memdump，本轮未用真实内存镜像验证。
- Native Windows 后端仍按 PRD 放到 V1.2，当前明确报未支持。

## 2026-05-05 21:45 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`。
- 本轮需求是窄范围 UI/品牌调整，不影响 PRD 功能链路。

### 当前改动

- 将启动 logo 的 ASCII art 改为 `QAQ`。
- 保留原有渐变输出、版本行和 JSON 模式不打印 logo 的行为。
- 调整 logo 宽度计算为 rune 维度，避免 Unicode block 字符导致渐变列计算偏差。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe parse --banner-file testdata\banners\ubuntu_5.4.0_163.txt`

### 后续风险

- 无功能风险；仅终端视觉变化。

## 2026-05-05 21:55 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前 `README.md`。
- 用户要求 banner 输入不再依赖 txt 文件，因此默认使用终端交互输入。

### 当前改动

- 新增终端 banner 读取逻辑：`volsym parse` 无参数时提示用户粘贴 banner 并按 Enter。
- `volsym build` 无 banner 文件、无 mem、无 ddeb/vmlinux、无手工 kernel/pkgver 时，改为从终端读取 banner。
- JSON 模式不输出交互提示，保证管道输入时 stdout 仍是纯 JSON。
- 保留 `--banner` 和 `--banner-file` 兼容旧流程，但 README 默认示例改为终端输入。
- 新增 `cmd/input_test.go` 覆盖 banner 读取与空输入错误。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `Get-Content testdata\banners\ubuntu_5.4.0_163.txt | go run . --json parse`
- `Get-Content testdata\banners\ubuntu_5.4.0_163.txt | go run . --json build --dry-run`
- `Get-Content testdata\banners\ubuntu_5.4.0_163.txt | go run . parse`

### 后续风险

- Windows 终端交互粘贴单行 banner 已覆盖；多行 banner 当前只读取第一行，因为 Linux kernel banner 本身按 PRD 是单行。

## 2026-05-05 22:05 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、`cmd/build.go`、下载器与 `README.md`。
- 用户反馈 ddeb 下载在读取 body 时超时；确认原实现使用 60 秒 `http.Client.Timeout`，不适合大 ddeb。

### 当前改动

- 新增 `build --download-timeout` 参数，默认 `30m`。
- 支持 `--download-timeout 2h` 等 Go duration 格式；传 `0` 表示不限制总下载时间。
- 下载器默认不再内置 60 秒总超时，由调用方传入策略。
- 下载失败错误信息现在带 `timeout=` 并提示可用 `--download-timeout 2h`。
- README 增加慢网络下载超时示例。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `Get-Content testdata\banners\ubuntu_5.4.0_163.txt | go run . --json build --dry-run --download-timeout 2h`

### 后续风险

- 这次修复下载总超时过短问题；若用户网络中断，仍会失败，但重试可命中 `.part` 以外的完整缓存文件。
- 断点续传仍未实现，按 PRD 属于 V1.1。

## 2026-05-05 22:15 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、下载器、`build` 命令与 README。
- 用户要求增加下载进度条，属于下载 UX 增强，不改变 resolver 或符号生成链路。

### 当前改动

- 下载器新增 `Progress` 与 `ProgressFunc` 回调。
- `build` 普通模式下载 ddeb 时在 stderr 显示进度条。
- 进度条在已知 `Content-Length` 时显示百分比、进度条、已下载/总大小。
- 未知总大小时显示已下载字节数。
- JSON 模式禁用进度条，保证 stdout 仍为纯 JSON。
- README 增加下载进度条说明。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- 单元测试覆盖下载进度回调被调用。

### 后续风险

- 未拉取真实大 ddeb 做人工视觉验收，避免默认下载数百 MB 文件。
- 进度条不会写入日志文件，目前仅终端显示。

## 2026-05-05 22:25 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与下载进度条实现。
- 用户要求把进度条尖端换成猫猫头，属于终端 UI 调整。

### 当前改动

- 下载进度条的当前位置尖端改为 `🐱`。
- 进度条已完成部分用 `=`，猫猫头后方留空。
- 满进度时猫猫头停在最右侧。
- README 示例更新为带猫猫头的进度条。
- 新增测试确认进度条包含 `🐱` 和百分比。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`

### 后续风险

- 终端字体若不支持 emoji，猫猫头可能显示为方框；功能不受影响。

## 2026-05-05 22:30 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与进度条实现。
- 用户要求不要 emoji，改用颜文字。

### 当前改动

- 下载进度条尖端从 `🐱` 改为 ASCII 颜文字 `(=^.^=)`。
- 调整进度条宽度，保证多字符尖端移动时总宽度稳定。
- README 示例同步更新。
- 测试改为确认包含 `(=^.^=)` 且不包含 emoji。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`

### 后续风险

- 颜文字尖端更宽，进度条占用列数略增加。

## 2026-05-05 22:40 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、WSL 后端与 runner。
- 用户贴出的错误显示 `bash -lc` 启动了污染的 WSL 登录环境，zsh/补全脚本干扰了构建脚本执行。

### 当前改动

- WSL 后端从 `bash -lc` 改为 `wsl --exec bash --noprofile --norc -c`。
- 避免读取 `.bashrc`、`.profile`、`.zshrc` 等用户 shell 初始化脚本。
- 新增 `bashArgs` 测试，锁定 non-interactive clean bash 调用参数。
- runner 新增 `CombinedOutputDisplay`，错误信息中只显示 `<script>` 摘要，不再把整段构建脚本塞进 `[ERROR]`。
- `doctor` 也使用同一 clean bash 路径，减少 WSL 环境噪声影响。

### 测试

- `go test ./...`
- `go run . doctor --json`
- `go build -o volsym.exe .`

### 后续风险

- WSL 自身的系统级 warning 仍可能输出，但不会再加载用户 shell 配置。
- 如默认 WSL distro 不是 Ubuntu，仍建议构建时显式传 `--wsl-distro Ubuntu`。

## 2026-05-05 22:46 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、WSL 后端、runner、build 命令与 README。
- 用户要求为后续解包和压缩阶段增加进度条，并将尖端改为奔跑的像素小猫；结合上一轮“不要 emoji”要求，本轮采用纯 ASCII 像素猫帧。

### 当前改动

- runner 新增 `StreamOutputDisplay`，支持实时读取 stdout/stderr、回调每行输出，同时保留完整输出用于错误信息和 marker 解析。
- WSL 后端新增 `BashStream`，在构建脚本输出 `VOLSYM_STAGE=...` 时回传阶段事件。
- ddeb 构建脚本新增 `extract`、`find_vmlinux`、`dwarf2json`、`compress`、`move`、`done` 阶段标记。
- vmlinux 构建脚本新增 `dwarf2json`、`compress`、`move`、`done` 阶段标记。
- build 普通模式新增解包、dwarf2json、压缩、收尾阶段的不定长进度条。
- 下载进度条与阶段进度条统一使用 ASCII 像素小猫帧：`=^._.^=>`、`=^._.^=>>`、`=^._.^=~`。
- JSON 模式不启用进度条和阶段回调，继续保证 stdout 是纯 JSON。
- README 更新进度条说明与示例。
- 新增测试覆盖 runner 流式输出、WSL 阶段标记、ASCII 小猫进度条与无 emoji 约束。

### 测试

- `go test ./...`

### 后续风险

- 解包、`dwarf2json` 和 `xz` 本身不提供细粒度百分比，本轮阶段进度条为不定长活动指示，不代表真实完成百分比。
- 未拉取真实大 ddeb 做端到端视觉验收，避免默认下载数百 MB 文件。

## 2026-05-05 23:07 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、进度条实现、构建测试与 README。
- 用户明确要求解包/压缩显示整体构建进度，不要表现为单个文件进度；小猫最终选择短、稳定、像素感强且不撑爆 Windows 终端的 ASCII 草案。

### 当前改动

- WSL 阶段进度从单阶段活动条调整为 `Building symbols` 整体构建进度。
- 整体进度按构建链路分段估算：`extract`、`find_vmlinux`、`dwarf2json`、`compress`、`move`。
- 进度条尖端改回短稳定 ASCII 像素小猫帧：`=^..^=__/`、`=^..^=__\`、`=^..^=__~`。
- README 示例同步更新为整体构建进度。
- 测试同步更新，继续确认不使用 emoji。

### 测试

- `go test ./...`

### 后续风险

- `dwarf2json` 和 `xz` 不暴露真实内部进度，整体百分比是阶段区间估算，不代表精确字节或文件完成度。

## 2026-05-05 23:15 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、resolver、build 命令与 README。
- 用户反馈卡在 “探测 ddeb 是否存在”；复核确认近期 WSL/进度条改动未触碰 resolver，阻塞点来自原有 ddeb 探测固定超时且无可见进度。

### 当前改动

- resolver 新增 `ProbeEvent` 与 `ProbeFunc`，每个候选 URL 探测前回调当前序号、总数与 URL。
- build 新增 `--probe-timeout`，默认 `30s`；传 `0` 表示不限制探测总时长。
- 普通模式新增 `Probing ddeb` 进度行，显示候选序号与当前包名。
- 探测失败错误信息带 `timeout=`，提示可用 `--probe-timeout 2m` 或手工 `--ddeb-url / --ddeb`。
- README 增加探测超时示例和探测进度示例。
- 新增测试覆盖 resolver 探测回调与 CLI 探测进度格式。

### 测试

- `go test ./...`

### 后续风险

- 探测仍依赖用户当前网络到 `ddebs.ubuntu.com`；代理、IPv6 或运营商网络问题仍可能导致超时，但现在有可见进度与可调超时。

## 2026-05-05 23:21 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、WSL 后端、进度条实现、构建测试与 README。
- 用户要求解包时使用双进度条：上方显示整体进度，下方显示当前文件解包进度。

### 当前改动

- ddeb 解包从 `dpkg-deb -x` 改为 `dpkg-deb --fsys-tarfile "$DDEB" | tar -xvf - -C "$WORK_DIR/extract"`。
- 解包前使用 `dpkg-deb -c` 统计 ddeb 内非目录条目数量，并输出 `VOLSYM_EXTRACT_TOTAL`。
- `tar -xvf` 每解包一个非目录条目输出 `VOLSYM_EXTRACT_FILE=current/total:path`。
- WSL 后端新增 `Extract` 回调，解析解包总数、当前序号和当前文件路径。
- 普通模式在 `extract` 阶段显示双进度条：上方 `Building symbols`，下方 `Extract files`。
- 非解包阶段自动回到单进度条，JSON 模式仍不显示进度。
- README 示例新增双进度条展示。
- 新增测试覆盖解包 marker、marker 解析、双进度条格式、非解包阶段隐藏下方进度条。

### 测试

- `go test ./...`

### 后续风险

- 下方解包进度按文件计数推进，不是当前单文件字节级进度；`dpkg-deb/tar` 不提供稳定的单文件字节进度。

## 2026-05-05 23:26 CST - Codex

### 文档评审

- 本轮继续检查双进度条改动后的依赖一致性。
- 发现解包脚本新增 `tar`，但 doctor 与 README 依赖列表未同步。

### 当前改动

- `doctor` 的 WSL 依赖检查新增 `tar`。
- README 的 WSL 安装命令新增 `tar`。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe doctor --json`

## 2026-05-05 23:40 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、logo、进度条实现与 README。
- 用户要求将 logo 改成 `meow~`，并汉化提示字符，同时说明后续如何修改提示字符。

### 当前改动

- 启动 logo 从 `QAQ` 改为 `MEOW~` ASCII art，副标题改为 `meow~ Vol3 Linux Symbol Builder v0.1.0`。
- 进度提示汉化：`探测 ddeb`、`下载中`、`构建符号`、`解包文件`、`解包 ddeb`、`查找 vmlinux`、`运行 dwarf2json`、`压缩 ISF`、`收尾`。
- README 示例同步改为中文提示。
- 测试同步改为检查中文提示。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe cache path`
- `Get-Content -Encoding UTF8 testdata\banners\ubuntu_5.4.0_163.txt | .\volsym.exe --json build --dry-run`

## 2026-05-05 23:45 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前 logo 实现。
- 用户反馈当前 `MEOW~` ASCII logo 在窄终端里呈现过高，要求改成横向展示。

### 当前改动

- 将启动 logo 改为单行横向样式：`=^..^=__/  meow~  Vol3 Linux Symbol Builder v0.1.0`。
- 移除重复副标题，避免一行 logo 后再额外输出标题。
- 保留原有渐变着色逻辑与 JSON 模式不输出 logo 的行为。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe cache path`
- `Get-Content -Encoding UTF8 testdata\banners\ubuntu_5.4.0_163.txt | .\volsym.exe --json build --dry-run`

### 后续风险

- 横向 logo 是固定字符串；如后续需要更短或换猫形，只需修改 `internal/logo/logo.go` 的 `logoLines`。

## 2026-05-05 23:48 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前 logo 实现。
- 上轮将“横着的”误处理成单行小 logo，导致用户期望的大 logo 消失；本轮纠正为横向大 logo。

### 当前改动

- 恢复多行大 `MEOW~` logo，改用较窄的横向 ASCII 字形。
- 保留底部短猫与产品名：`=^..^=__/  meow~ Vol3 Linux Symbol Builder v0.1.0`。
- JSON 模式仍不输出 logo，普通模式保留渐变色。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe cache path`
- `Get-Content -Encoding UTF8 testdata\banners\ubuntu_5.4.0_163.txt | .\volsym.exe --json build --dry-run`

### 后续风险

- 该大 logo 宽度约 39 列，正常 Windows 终端不会竖向挤压；如果用户终端更窄，仍可能自动换行。

## 2026-05-05 23:53 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前 logo 实现。
- 用户提供截图，要求恢复截图中的粗体大 `MEOW` 风格，但将波浪号改成横向展示。

### 当前改动

- 将 logo 改回粗体大块 `MEOW` 风格。
- 将右侧波浪号改为中线横向 `~~~~~~`，避免再生成竖向波浪图形。
- 保留底部短猫与产品名：`=^..^=__/  meow~ Vol3 Linux Symbol Builder v0.1.0`。
- JSON 模式仍不输出 logo，普通模式保留渐变色。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe cache path`
- `Get-Content -Encoding UTF8 testdata\banners\ubuntu_5.4.0_163.txt | .\volsym.exe --json build --dry-run`

### 后续风险

- 当前波浪号是普通 ASCII `~~~~~~`，终端字体兼容性最好；如果要做块状横向波浪，需要再次微调 `internal/logo/logo.go` 的 `logoLines`。

## 2026-05-05 23:58 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前 logo 实现。
- 用户要求波浪线也使用大符号块，并明确需要横向大符号块。

### 当前改动

- 将右侧普通 `~~~~~~` 改为与 `MEOW` 同高度的块状横向波浪。
- 保持 `MEOW` 粗体大块字风格、渐变输出和底部短猫产品名。
- 控制 logo 最大行宽约 66 列，避免常见 80 列 Windows 终端自动换行。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe cache path`
- `Get-Content -Encoding UTF8 testdata\banners\ubuntu_5.4.0_163.txt | .\volsym.exe --json build --dry-run`

### 后续风险

- 大块横向波浪仍依赖终端字体对 box/block 字符的渲染；Windows Terminal 默认等宽字体显示正常。

## 2026-05-05 23:59 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前 logo 实现。
- 用户要求底部小 logo 字体也改成彩色。

### 当前改动

- 抽出 `printGradientLine`，大 logo 与底部小猫产品名共用同一套渐变着色。
- 底部 `=^..^=__/  meow~ Vol3 Linux Symbol Builder v0.1.0` 现在也会逐字符输出 ANSI RGB 渐变。
- JSON 模式仍不输出 logo，保持纯 JSON。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe cache path`
- `Get-Content -Encoding UTF8 testdata\banners\ubuntu_5.4.0_163.txt | .\volsym.exe --json build --dry-run`

### 后续风险

- 小 logo 彩色输出复用 ANSI 真彩色；不支持 ANSI 的终端会看到转义码，和大 logo 行为一致。

## 2026-05-06 00:20 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、banner/resolver/build 实现与 README。
- 用户要求新增常见服务器发行版支持；结合当前 WSL 后端只能直接解 `.deb/.ddeb`，本轮优先扩展 banner 识别与 Debian `.deb` 自动候选，不伪装支持 RPM 自动下载。

### 当前改动

- 新增 `banner.ParseBanner` 统一入口，自动识别 Ubuntu、Debian、RHEL/CentOS/Rocky/Alma、Amazon Linux、SUSE/openSUSE 常见服务器 banner。
- 新增 Debian banner 解析，能从 `#... Debian 5.10.237-1` 提取 kernel package version，并避免误抓 GCC 的 `Debian 10.2.1-6`。
- Debian 自动生成候选：`linux-image-<kernel>-dbg_<pkgver>_<arch>.deb` 与 `linux-image-<kernel>-dbgsym_<pkgver>_<arch>.deb`。
- RHEL/CentOS/Rocky/Alma/Amazon/SUSE 目前只识别 distro/kernel/arch；自动下载返回空候选，需用 `--vmlinux`、`--ddeb-url` 或 `--ddeb` 手工构建。
- `parse` 与 `build` 从 `ParseUbuntuBanner` 切到 `ParseBanner`。
- `--distro` 默认值从 `ubuntu` 改为空，避免覆盖 banner 自动识别结果；手工模式仍由 `symbols.MergeManual` 默认到 Ubuntu。
- 本地 debug package 文件名推断新增 Debian `.deb`：`linux-image-...-(dbg|dbgsym)_..._arch.deb`。
- README 支持范围改为 Ubuntu 自动、Debian 候选、RPM 系识别+手工构建。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- Debian banner：`.\volsym.exe --json parse`
- Debian banner：`.\volsym.exe --json build --dry-run`
- RHEL/CentOS 样例 banner：`.\volsym.exe --json parse`
- RHEL/CentOS 样例 banner：`.\volsym.exe --json build --dry-run`
- Ubuntu 回归：`Get-Content -Encoding UTF8 testdata\banners\ubuntu_5.4.0_163.txt | .\volsym.exe --json build --dry-run`

### 后续风险

- Debian 旧安全内核可能已从当前 `deb.debian.org/debian/pool/main/l/linux` 移走，需要用户从 `snapshot.debian.org` 找包后用 `--ddeb-url`。
- RPM 系 debug 包需要 `rpm2cpio/cpio` 解包或直接提供 `vmlinux`；本轮未实现 RPM 自动下载/解包。

## 2026-05-06 01:33 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前代码。
- 用户要求实行开源符号库 TXT 配置方案，并在编译时将 `appicon.png` 作为 Windows exe 图标。
- 计划中“默认源 Abyss-W4tcher”经实测确认：`banners_plain.json` 的值为路径数组，因此实现需兼容 `banner -> []path`，不能只按 `map[string]string` 解析。

### 当前改动

- 新增 `internal/symbolsources`，支持 `%USERPROFILE%\.volsym\symbol-sources.txt`，格式为 `name|index_url|raw_base_url`，支持注释、空行与行号报错。
- 默认内置 Abyss-W4tcher 源；文件不存在时自动使用内置默认源。
- `parse` 增加 `--symbol-sources`、`--no-remote-symbols`，JSON 输出 `symbol_sources_path`、`symbol_sources`、`remote_symbol_candidates`、`support_level` 等字段，并对 banner 做远程索引精确匹配。
- `build` 增加 `--symbol-sources`、`--no-remote-symbols`、`--debug-package`、`--debug-package-url`；保留 `--ddeb`、`--ddeb-url` 兼容别名。
- `build` 流程改为远程 ISF 优先：命中则下载 `.json.xz` 到 `symbols/linux/` 并直接结束；未命中再回退 Ubuntu/Debian debug package 自动候选或 RPM 手工链路。
- 新增包格式识别：`ddeb`、`deb`、`rpm`、`vmlinux`、`isf`、`unknown`。
- WSL 后端支持 RPM 本地 debug package 解包：`.rpm` 走 `rpm2cpio | cpio -idmv`；`.deb/.ddeb` 保留 `dpkg-deb --fsys-tarfile | tar`。
- WSL 搜索路径扩展到 `/usr/lib/debug/boot/vmlinux-*`、`/usr/lib/debug/lib/modules/<kernel>/vmlinux`、`/usr/lib/debug/lib64/modules/<kernel>/vmlinux`，并支持 `vmlinux.gz/.xz/.zst` 解压。
- `doctor` 新增检查 `rpm2cpio`、`cpio`、`gzip`、`zstd`。
- `config init` 现在同时写出 `config.json` 与 `symbol-sources.txt`；`config show` 显示 `symbol_sources_path`。
- README 更新远程符号源、TXT 配置、RPM 本地包、兼容别名与 WSL 依赖说明。
- 新增 `winres/winres.json`、`winres/appicon-256.png`、`rsrc_windows_amd64.syso`，使用 `appicon.png` 生成 256x256 图标资源并嵌入 `volsym.exe`。

### 测试

- `go test ./...`
- Debian banner：`.\volsym.exe --json parse --no-remote-symbols`
- RHEL banner：`.\volsym.exe --json build --dry-run --no-remote-symbols`
- `go build -o volsym.exe .`
- PowerShell `VersionInfo` 验证 `volsym.exe` 包含 `ProductName=volsym`、`CompanyName=QAQ`、`FileDescription=Vol3 Linux Symbol Builder`。

### 后续风险

- RPM 公开仓库 `repodata/repomd.xml` 自动下载尚未接入 CLI；当前 RPM 系远程未命中后仍需 `--debug-package`、`--debug-package-url` 或 `--vmlinux`。
- `doctor` 会把 RPM 工具缺失视作构建环境不完整；若用户只构建 Ubuntu/Debian，可忽略 `rpm2cpio/cpio/zstd` 缺失。
- 图标资源由 `appicon.png` 派生 `winres/appicon-256.png`；若以后更新原图，需要重新生成该派生图并运行 `go-winres make --arch amd64 --out rsrc`。

## 2026-05-06 01:38 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前代码。
- 上轮记录的主要缺口是 RPM 公开仓库 `repodata/repomd.xml` 自动下载尚未接入 CLI；本轮优先补该可落地缺口，并清理仍写死 `ddeb` 的提示。

### 当前改动

- `build` 新增 `--repo-url`，用于用户提供公开或内网 RPM repo base；流程会读取 `repodata/repomd.xml` 与 primary metadata，按 RPM debug package 文件名精确匹配。
- RPM repo 命中后：dry-run 输出 `found_url`；真实构建会下载 RPM 后进入现有 WSL RPM 解包链路。
- RPM 候选名修正：当 kernel release 已带 `.x86_64` / `.aarch64` 后缀时，生成包名时不再重复追加 arch，避免 `...x86_64.x86_64.rpm`。
- CentOS/Rocky/Alma/Fedora banner 识别细化；含 Rocky/Alma/Fedora 构建主机或 `fcNN` release 时不再一律归为 RHEL。
- 进度文案从 `探测 ddeb` / `解包 ddeb` 泛化为 `探测包` / `解包调试包`，避免 RPM 路径显示误导。
- README 更新 `--repo-url` 与 RPM repo metadata 说明。

### 测试

- `go test ./...`
- 新增 resolver fixture：`repomd.xml + primary.xml.gz`，断言能命中 `kernel-debuginfo` URL。
- 新增 RPM 候选名测试，覆盖 kernel release 自带 arch 后缀。
- 新增 Rocky/Alma/Fedora banner 识别测试。

### 后续风险

- 当前 `--repo-url` 只负责用户给 repo base 的精确查找；仍不内置各发行版镜像 URL 推断，避免猜错闭源/订阅或企业源。
- primary metadata 当前支持 XML 与 `.gz`；若发行版只提供 SQLite primary DB，仍需后续扩展。

## 2026-05-06 14:43 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前 CLI help。
- 用户截图指出顶层命令列表仍是旧文案，且 help 时会先输出大 logo，影响阅读。

### 当前改动

- 顶层 help 改为新版命令说明：
  - `build`：生成/下载 Volatility 3 Linux ISF 符号表
  - `cache`：查看/清理下载缓存
  - `config`：查看/初始化配置与符号源
  - `doctor`：检查 Windows/WSL/dwarf2json 依赖
  - `parse`：解析 banner 并查询远程符号源
  - `verify`：用 Volatility 3 验证符号目录
- 顶层用法改为 `volsym [全局参数] <命令> [命令参数]`。
- 全局参数说明补充 `--json` 会输出纯 JSON 且不显示 logo/进度条。
- help 增加常用命令示例：`parse`、`build --dry-run`、`build --debug-package local.rpm`、`build --repo-url ...`。
- `main.go` 在 `-h/--help` 下不再打印大 logo，避免 help 输出被 logo 挤占。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe --help`
- `.\volsym.exe -h`
- PowerShell `VersionInfo` 验证图标/版本资源仍存在。

### 后续风险

- 子命令自己的 `-h` 仍由 Go `flag` 默认格式输出；若需要统一美化，还需后续定制每个子命令 help。

## 2026-05-06 14:53 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md` 与当前 `cmd/root.go`。
- 用户指出全局参数展示少于实际解析支持项；确认实际支持 `-h/--help`，但 help 列表未展示。

### 当前改动

- 顶层 help 的全局参数补充 `-h, --help 显示帮助`。
- 兼容 `-help`，避免用户按 Go flag 常见写法输入时被当作未知全局参数。
- `main.go` 同步在 `-help` 下不打印 logo。
- 抽出 `printUsage()`，便于测试直接断言 help 文案。
- 新增 `cmd/root_test.go`，覆盖 help 文案必须列出 `--verbose`、`--json`、`-h, --help`，并覆盖 `-h/--help/-help` 都触发 usage。

### 测试

- `go test ./...`
- `go build -o volsym.exe .`
- `.\volsym.exe --help`
- `.\volsym.exe -help`
- PowerShell `VersionInfo` 验证图标/版本资源仍存在。

### 后续风险

- 子命令 flag 中也有 `--json/--verbose` 等局部兼容项；顶层 help 只列 root 预解析支持的全局参数，未展开每个子命令参数。

## 2026-05-06 15:15 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、README、root help、Windows resource 配置与当前命名引用。
- 用户要求将 `volsym` 改为 `meow`，并同步更新 README。

### 当前改动

- Go module 从 `volsym` 改为 `meow`，项目内部 import 同步更新。
- 可见 CLI 名称从 `volsym` 改为 `meow`：
  - 顶层 help 用法改为 `meow [全局参数] <命令> [命令参数]`。
  - 常用示例改为 `meow parse`、`meow build ...`。
  - verify 错误建议改为 `meow cache clear`。
- 默认用户目录从 `%USERPROFILE%\.volsym` 改为 `%USERPROFILE%\.meow`，覆盖 cache、config、symbol-sources 默认路径。
- Windows resource 更新：
  - `ProductName=meow`
  - `InternalName=meow`
  - `OriginalFilename=meow.exe`
- 编译产物改为 `meow.exe`，旧 `volsym.exe` 已移除。
- README 全面改为 `meow` / `meow.exe` / `.meow`，并补充旧 `.volsym` 目录需手工迁移说明。
- README 同步修正进度文案为 `探测包` / `解包调试包`，并修正 RPM 示例包名不再重复 arch。
- 保留内部 WSL marker `VOLSYM_*`，避免误伤构建脚本和既有测试；这是内部协议名，不作为用户可见品牌。

### 测试

- `go test ./...`
- `go build -o meow.exe .`
- `.\meow.exe --help`
- `.\meow.exe --json cache path`，确认默认路径为 `%USERPROFILE%\.meow\cache`
- PowerShell `VersionInfo` 验证 `ProductName=meow`、`InternalName=meow`、`OriginalFilename=meow.exe`。

### 后续风险

- 旧 `%USERPROFILE%\.volsym` 配置和缓存不会自动迁移；README 已说明需手工复制到 `.meow`。
- PRD 仍保留最初项目名 `volsym` 作为历史需求文档，未批量改写。

## 2026-05-08 17:56 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、`README.md` 与当前 `cmd/internal` 代码。
- 评审结论：当前文档和实现仍大量描述 WSL 透传模型（`--backend` / `--wsl-distro` / `wsl.exe`），与本轮“仅 Linux 原生运行”决策冲突，需要整体迁移。

### 当前改动

- 后端重构为 Linux 原生执行：
  - 删除 `internal/backend/wsl.go` 与对应测试。
  - 新增 `internal/backend/native.go`、`internal/backend/native_test.go`。
  - `build/doctor` 不再调用 `wsl.exe`，改为 `bash --noprofile --norc -c` 本机执行。
  - 保留阶段与解包进度 marker 协议：`VOLSYM_STAGE` / `VOLSYM_EXTRACT_*`，现有双进度条逻辑无需改动。
- CLI 破坏性变更（按锁定决策）：
  - 从 `build`/`doctor` 移除 `--backend`、`--wsl-distro`。
  - 非 Linux 平台在 `build` 与 `doctor` 入口统一快速失败：`当前版本仅支持 Linux 原生运行`。
  - `build` JSON 中 `backend` 固定为 `linux_native`。
- 配置变更：
  - `config` 结构与 `show/init` 输出删除 `backend`、`wsl_distro` 字段。
  - 保留 `cache_dir`、`output_dir`、`symbol_sources_path`、`volatility_path` 等有效项。
- CI 基线接入：
  - 新增 `.github/workflows/ci.yml`（`ubuntu-latest`）。
  - 固定执行：依赖安装、`go test ./...`、`go build -o meow .`、3 条 `--no-remote-symbols` smoke（parse / Ubuntu dry-run / RPM-family dry-run）。
- 文档迁移：
  - `README.md` 全量改为 Linux 原生语义，去除 WSL 专属命令和排障路径。
  - 命令示例统一移除 `--backend wsl`。

### 测试

- Windows 本地编译与单测：
  - `go test ./...`
  - `go build -o meow.exe .`
- Linux 语义 smoke（在 WSL Ubuntu 内执行 Linux 二进制）：
  - `./meow --json parse --banner-file ./testdata/banners/ubuntu_5.4.0_163.txt --no-remote-symbols`
  - `./meow --json build --dry-run --banner-file ./testdata/banners/ubuntu_5.4.0_163.txt --no-remote-symbols`
  - `./meow --json build --dry-run --banner-file ./testdata/banners/centos_4.18.0_513.txt --no-remote-symbols`
- CLI 兼容性验证：
  - `meow build --help` / `meow doctor --help` 已无 `--backend`、`--wsl-distro`。
  - 新增 `cmd/command_flags_test.go` 断言旧参数已移除。

### 后续风险

- 旧自动化脚本若仍传 `--backend` 或 `--wsl-distro` 将直接报 `flag provided but not defined`，需要手工迁移脚本参数。
- CI 当前按方案只做单元与干跑烟测，不覆盖真实大包下载/解包/`dwarf2json` 全链路。
- `winres` 与 Windows 图标资源文件仍保留在仓库中，但不纳入 Linux CI 运行路径。

## 2026-05-15 23:20 CST - Codex

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、`README.md`、`PRD.md` 与当前源码。
- `README.md` 已明确当前版本是 Linux 原生运行，并写清非 Linux 构建链路会失败。
- `PRD.md` 仍大量保留早期 Windows/WSL MVP 描述，和当前 Linux-only 实现存在明显冲突；建议后续将其标记为历史 PRD，或新增/重写当前 Linux 原生版 PRD，避免后续开发误按旧 WSL 方案验收。

### 审计结论

- 高风险：`cache clear --cache-dir <path>` 最终会对用户传入路径执行递归删除；当前只拒绝空路径、`.` 和根目录，仍可能误删 `$HOME`、项目目录或其他非缓存目录。
- 高风险：debug package 解包依赖 `tar` / `cpio` 直接落盘，当前没有对包内成员路径做显式校验；若用户使用不可信 `--debug-package-url` 或恶意 repo，存在路径穿越/覆盖工作目录外文件的风险，需要在解包前拒绝绝对路径与 `..` 成员，或改为安全解包实现。
- 中风险：远程符号源索引和 RPM metadata 使用 `io.ReadAll` 无大小上限；恶意或异常大的 `symbol-sources.txt` / `--repo-url` 响应可能导致内存耗尽。
- 中风险：`repomd.xml` 中 `primary_db` 会被当成可用 primary metadata，但后续只按 XML/`.gz` 解析，若仓库优先返回 SQLite primary DB 会解析失败。
- 低风险：`config init/show` 目前只创建和展示配置，`build/parse/cache/verify` 并未读取 `config.json` 中的默认值；用户可能以为配置已生效。
- 低风险：`doctor` 当前不检查 `vol`，但 `verify` 实际依赖 Volatility 3；普通输出中的 verify 可用性判断容易误导。
- 仓库卫生：当前工作区已有非本轮产生的删除状态 `appicon.png`、`meow.exe`，且仓库跟踪了 Linux ELF 构建产物 `meow` 与 Windows 资源/二进制相关文件；后续建议决定是否保留发布制品在源码仓库中，并补充 `.gitignore`。

### 测试

- `go test ./...`
- `go vet ./...`
- `go run . --json parse --banner-file testdata\banners\ubuntu_5.4.0_163.txt --no-remote-symbols`
- `go run . --json build --dry-run --banner-file testdata\banners\ubuntu_5.4.0_163.txt --no-remote-symbols`：在 Windows 本地按 Linux-only 守卫返回 `当前版本仅支持 Linux 原生运行`，符合当前 README 支持范围。

### 后续建议

- 优先修复递归删除保护与安全解包边界，这两项最接近真实误操作/恶意输入风险。
- 给远程索引和 RPM metadata 增加大小上限、内容类型/压缩格式约束与测试。
- 决定 `config.json` 是要真正参与命令默认值，还是仅作为展示/模板；若暂不生效，应在 README 中说清。

## 2026-06-05 21:10 CST

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、`README.md`、`PRD.md` 与当前源码。
- 上轮审计结论中的高风险项（递归删除保护、安全解包边界）仍未修复，但不在本轮范围内。
- 本轮目标：整理 .gitignore、提交 Linux-native 重构、添加 TUI 子项目。

### 当前改动

- 新增 `.gitignore`：排除 `meow`/`meow.exe` 二进制、`node_modules/`、`.claude/`、`.omc/`。
- 从 git 追踪中移除 `meow`、`meow.exe`、`appicon.png`。
- 提交 Linux-native 重构（10 文件变更）：
  - `cmd/build.go`：用 `buildInput` 结构体替代 11 值返回元组。
  - `internal/backend/native.go`：嵌入 bash 脚本，删除 `wsl.go`。
  - `internal/backend/scripts/debug_package.sh`：RPM/deb 解压、vmlinux 查找、dwarf2json、xz 压缩。
  - `internal/backend/scripts/vmlinux.sh`：直接 vmlinux 输入时的 dwarf2json + 压缩。
  - `internal/resolver/ddeb.go`：HTTPS 升级（`http://` → `https://`）。
  - `cmd/verify.go`：`exec.LookPath` 前置检查 vol。
  - `.github/workflows/ci.yml`：Ubuntu-latest CI 门禁。
- 提交 `golangci-lint` 配置（`.golangci.yml`）。
- 提交 CLAUDE.md：为 Claude Code 提供项目指引。

### 测试

- `go test ./...` — 全部通过。
- `go build -o meow .` — Linux ELF 编译成功。

## 2026-06-05 21:30 CST

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、OpenTUI 组件文档、当前 Go 后端代码。
- 编译环境确认为 WSL/Linux，TUI 子项目假设直接在 Linux 环境运行，不调用 `wsl.exe`。
- 设计决策：TUI 不复制 Go 逻辑，仅通过 `Bun.spawn` 调用 `meow` 和 `vol` 子进程。

### 当前改动

- 新增 `tui/` 子项目（Bun + TypeScript + @opentui/core）：
  - `tui/package.json`：依赖 `@opentui/core ^0.3.2`、`@types/bun ^1.3.14`、`typescript ^6.0.3`。
  - `tui/tsconfig.json`：ES2022、strict mode、bundler moduleResolution。
  - `tui/src/types.ts`：`AppState`、`ScreenName`、`LogLevel`、`LogEntry`、`RunningTask`。
  - `tui/src/state.ts`：不可变状态变更函数（`appendLog`、`appendChunkLog`、`setRunningTask` 等）。
  - `tui/src/screens.ts`：7 个页面定义（Dashboard/Parse/Build/Volatility/Workflow/Cache/Logs）。
  - `tui/src/app.ts`：OpenTUI 渲染器、键盘处理、ScrollBox 主内容区、日志预览。
  - `tui/src/runner/process.ts`：通用 `Bun.spawn([cmd, ...args])` 封装，支持 AbortSignal 和流式日志。
  - `tui/src/runner/meow.ts`：meow CLI 封装（`runMeowJSON`、`runDoctor`、`runParseBannerFile`、`runBuildDryRun`、`runVerify`、`runCacheList`）。
  - `tui/src/runner/vol.ts`：Volatility 3 封装（`runVolPlugin`、`extractBanner`、`runPsList`）。
  - `tui/tests/runner.test.ts`：参数构建测试（3 个用例）。
- 实现 MVP Workflow：vol banner 提取 → meow dry-run build → meow verify。
- 实现命令快捷键：数字 1-7 切页面、`r` 执行、`x` 取消、`q/Esc` 退出。

### 测试

- `bun install --cwd tui` — 依赖安装成功。
- `bun test` — 3/3 通过。
- `tsc --noEmit` — 类型检查通过（修复 `titleColor` 属性不兼容后）。

### 已修复的验证问题

- OpenTUI 当前版本不支持 `titleColor` 属性，已从 `Box`/`ScrollBox` 配置中移除。
- `go test ./...` 在非 repo root 执行时报 `no packages to test`，改用 `go -C <repo-root> test ./...`。
- `bunx --cwd ... tsc` 路径解析失败，改用 `node_modules/.bin/tsc` 直接调用。

## 2026-06-05 21:35 CST

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、当前 TUI 实现与 OpenTUI 组件文档。
- TUI 工作区已有 `screens.ts`（7 页面）和 `app.ts`（命令式布局），本轮重构为三面板布局 + 命令系统。

### 当前改动

- 删除 `screens.ts`，重构为命令驱动的三面板布局：
  - 左面板 (22%)：镜像信息、符号表路径、输出目录、当前插件、Banner 信息、运行状态。
  - 中面板 (flexGrow)：日志输出 (ScrollBox)，底部粘性滚动。
  - 右面板 (34 cols)：插件分类列表，当前插件高亮。
- 新增 `tui/src/commands.ts`：`/` 前缀命令解析（`/mem`、`/symbol`、`/plugin`、`/run`、`/banner`、`/build`、`/verify`、`/clear`、`/help`）。
- 新增 `tui/src/plugins.ts`：Volatility 3 插件分类列表（Process/Memory/Network/FileSystem/Kernel）。
- 新增 `tui/src/logo.ts`：ASCII art `MEOW~~` logo 渲染。
- 重构 `app.ts`：
  - `Input` 组件用于命令输入，`i` 键聚焦，`Esc` 取消聚焦。
  - `actionMap` 模式替代 switch/case，闭包捕获 `AbortController`。
  - `setState` → `redraw()` 统一触发重绘。
- 重构 `state.ts`：新增 `setCommandInput`、`setInputFocused`、`setImageInfo`。
- 扩展 `types.ts`：新增 `ImageInfo`、`commandInput`、`inputFocused` 字段。

### 测试

- `bun test` — 3/3 通过。
- `tsc --noEmit` — 类型检查通过。

## 2026-06-05 21:45 CST

### 文档评审

- 本轮开始已审阅 7 个 commit 的完整 diff（4330856..09e43c8），共 30 文件、+1656/-180 行。
- 发现 1 个 CRITICAL、2 个 HIGH、4 个 MEDIUM、1 个 LOW 问题。

### 当前改动

**CRITICAL — debug_package.sh 提取失败被吞没**
- 问题：`cmd | while read` 管道中 `while` 循环退出码总是 0，即使 `cpio`/`tar` 失败。
- 修复：改为临时文件模式 — 先执行提取命令写入临时文件、检查退出码、再从文件逐行读取。
- RPM 分支和 deb/ddeb 分支均已修复。
- 提取失败时输出 `[ERROR] ... extraction failed (code N)` 并立即退出。

**HIGH — verify 结果覆盖 doctor 数据**
- 问题：`runAction("verify")` 将结果存入 `lastDoctorResult`，覆盖之前 doctor 的结果。
- 修复：`types.ts` 新增 `lastVerifyResult` 字段，verify action 和 workflow verify 步骤均使用新字段。

**MEDIUM — 移除冗余 `ddebBuildScript()`**
- 问题：`ddebBuildScript()` 和 `debugPackageBuildScript()` 返回相同内容，前者无外部调用。
- 修复：移除 `ddebBuildScript()`，测试改用 `debugPackageBuildScript()`。

**MEDIUM — package.json 依赖 `latest`**
- 问题：所有依赖版本为 `"latest"`，lockfile 再生时可能拉到不兼容版本。
- 修复：改为 `^0.3.2` / `^1.3.14` / `^6.0.3`。

**LOW — CI golangci-lint `version: latest`**
- 修复：固定为 `v2.1`。

### 测试

- `go test ./...` — 全部通过。
- `bun test` — 3/3 通过。
- `tsc --noEmit` — 类型检查通过。

### 未修复的低置信度问题

- `Bun.spawn` abort 时孙进程可能成为孤儿（需运行时确认 Bun 进程组 kill 行为）。
- `lookPathWithContext` goroutine 泄漏（`LookPath` 极快，实际无影响）。

## 2026-06-05 22:00 CST

### 文档评审

- 本轮开始已读取 `docs/development-log.md`、当前 CI 配置与项目结构。
- 当前 CI 只有一个 `test-build-smoke` job，覆盖 Go 后端但不覆盖 TUI 前端。
- 无架构设计文档，仅有开发日志和 PRD。

### 当前改动

**CI 门禁拆分**
- 将 `.github/workflows/ci.yml` 从单 job 拆为两个并行 job：
  - `backend (Go)`：lint → test → build → 3 条 smoke。
  - `frontend (TUI)`：`bun install --frozen-lockfile` → `bunx tsc --noEmit` → `bun test`。
- 前端 CI 使用 `oven-sh/setup-bun@v2` 固定 Bun 版本 `1.3.14`。
- 前端 CI `working-directory: tui`，步骤自动在子项目目录执行。

**架构设计文档**
- 新增 `docs/architecture.md`，覆盖：
  - 系统全景图（ASCII 模块关系图）
  - Go 后端架构：CLI 层、Build 数据流水线、Banner 解析算法、Resolver 探测策略、后端脚本 Marker 协议、包格式识别
  - TUI 前端架构：技术栈、UI 布局（三面板 ASCII 图）、状态管理、命令系统、Runner 层、生命周期
  - 关键设计决策：TUI 不复制逻辑、argv 数组、不可变状态、临时文件错误处理、HEAD→GET Range 探测、远程 ISF 优先
  - 测试策略

**开发日志续写**
- 追加 5 条日志条目：Linux-native 重构、TUI MVP、三面板重构、代码审阅修复、CI/文档补充。

### 测试

- `bun test && bunx tsc --noEmit` — 前端 CI 步骤本地验证通过。
- `go test ./...` — 后端测试通过。
