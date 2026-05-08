# MEOW~

制作 Volatility 3 Linux 符号表的小工具喵。  
当前版本已重构为 **Linux 原生运行**，不再调用 `wsl.exe`。

## 支持范围

- 运行环境：Linux（包括你在 WSL 发行版内直接运行 `./meow`）
- 输入方式：终端粘贴 banner、`--banner-file`、`--mem`、`--debug-package`、`--debug-package-url`、`--vmlinux`
- 远程 ISF：读取 `$HOME/.meow/symbol-sources.txt`，默认查询 Abyss-W4tcher 开源符号库
- Ubuntu：远程 ISF 未命中时自动探测 `.ddeb`
- Debian：远程 ISF 未命中时自动生成 `.deb` 候选并探测
- RHEL/CentOS/Rocky/Alma/Fedora/openSUSE：远程 ISF 优先；未命中时支持 `--repo-url`、`--debug-package`、`--debug-package-url`、`--vmlinux`

暂不支持：

- Windows 原生执行构建链路（非 Linux 会直接报错）
- 订阅/闭源仓库自动推断与绕过授权下载
- 无 debug package / 无 vmlinux 的符号恢复

## Linux 依赖

`doctor` 和构建链路需要以下工具：

```bash
sudo apt update
sudo apt install -y dpkg-dev xz-utils rpm2cpio cpio gzip zstd tar
```

`dwarf2json` 需要可执行并在 PATH 中：

```bash
git clone https://github.com/volatilityfoundation/dwarf2json
cd dwarf2json
go build -o dwarf2json
sudo cp dwarf2json /usr/local/bin/
```

检查环境：

```bash
./meow doctor
```

## 快速开始

```bash
./meow doctor
./meow parse
./meow build --out ./symbols/linux
./meow verify --mem ./memdump.mem --symbols ./symbols
```

`parse` 和默认 `build` 会提示在终端粘贴 Linux kernel banner，粘贴后按 Enter。

## 常用命令

只解析，不下载不构建：

```bash
./meow build --dry-run
```

JSON 输出：

```bash
./meow --json parse --banner-file ./testdata/banners/ubuntu_5.4.0_163.txt
./meow --json build --dry-run --banner-file ./testdata/banners/ubuntu_5.4.0_163.txt
```

禁用远程符号库：

```bash
./meow build --no-remote-symbols --dry-run
```

指定符号源 TXT：

```bash
./meow build --symbol-sources /path/to/symbol-sources.txt
```

## 构建流程说明

`build` 流程优先级：

1. 远程 ISF（命中则直接下载 `.json.xz`，停止后续下载 debug package）
2. 手工 URL（`--debug-package-url` / 兼容别名 `--ddeb-url`）
3. `--repo-url`（RPM repo metadata 精确查找）
4. 按发行版候选自动探测（Ubuntu/Debian）
5. 手工包或本地 vmlinux 兜底

下载与探测超时可调：

```bash
./meow build --probe-timeout 2m --download-timeout 2h --out ./symbols/linux
```

## 从内存镜像生成

```bash
./meow build --mem ./memdump.mem --out ./symbols/linux
```

工具会调用：

```text
vol -f memdump.mem banners.Banners
```

然后进入同一套自动解析与构建流程。

## 手工输入模式

提供本地 debug package：

```bash
./meow build --debug-package ./linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
./meow build --debug-package ./linux-image-5.10.0-35-amd64-dbg_5.10.237-1_amd64.deb
./meow build --debug-package ./kernel-debuginfo-4.18.0-513.5.1.el8_9.x86_64.rpm
```

本地包名无法推断 kernel/pkgver 时，补齐参数：

```bash
./meow build \
  --debug-package ./kernel-debug-package.deb \
  --kernel 5.10.0-35-amd64 \
  --pkgver 5.10.237-1 \
  --arch amd64 \
  --distro debian
```

提供本地 vmlinux：

```bash
./meow build --vmlinux ./vmlinux-5.4.0-163-generic --out ./symbols/linux
```

兼容别名：

- `--ddeb` == `--debug-package`
- `--ddeb-url` == `--debug-package-url`

## RPM 系说明

RHEL/CentOS/Rocky/Alma/Fedora/openSUSE 未命中远程 ISF 时：

- 可用 `--repo-url`（需包含 `repodata/repomd.xml`）
- 或直接 `--debug-package`
- 或 `--vmlinux`

示例：

```bash
./meow build --repo-url https://mirror.example.org/debug/os/x86_64/ --dry-run
./meow build --debug-package ./kernel-debuginfo-4.18.0-513.5.1.el8_9.x86_64.rpm
```

## 配置与符号源

查看默认配置：

```bash
./meow config show
```

初始化配置：

```bash
./meow config init
```

默认路径：

```text
$HOME/.meow/config.json
$HOME/.meow/symbol-sources.txt
```

`symbol-sources.txt` 格式（一行一个源）：

```text
# name|index_url|raw_base_url
abyss|https://raw.githubusercontent.com/Abyss-W4tcher/volatility3-symbols/master/banners/banners_plain.json|https://raw.githubusercontent.com/Abyss-W4tcher/volatility3-symbols/master/
```

若你历史使用过 `%USERPROFILE%\\.volsym`，请手工迁移 `config.json`、`symbol-sources.txt` 和缓存到新目录 `$HOME/.meow`。

## 缓存

```bash
./meow cache path
./meow cache list
./meow cache clear
```

## 给 Volatility 3 使用

生成结果示例：

```text
symbols/linux/Ubuntu_5.4.0-163-generic_5.4.0-163.180_amd64.json.xz
```

运行：

```bash
vol -f ./memdump.mem -s ./symbols linux.pslist.PsList
```

注意 `-s` 指向 `symbols` 父目录，不是 `symbols/linux`。

## 常见错误

### `当前版本仅支持 Linux 原生运行`

你在非 Linux 平台直接执行了构建链路。请在 Linux 环境运行（例如 WSL 发行版 shell 内执行 Linux 二进制）。

### 未找到对应 debug package

常见原因：

- 发行版仓库已滚动清理旧包
- banner 不属于当前支持自动探测范围
- 包版本解析与实际仓库不一致

处理方式：

```bash
./meow build --debug-package-url <url> ...
./meow build --debug-package ./local-package.rpm ...
./meow build --vmlinux ./vmlinux-... ...
```

### 未找到 `dwarf2json`

确认 `dwarf2json` 在 PATH：

```bash
command -v dwarf2json
```
