# MEOW~

制作vol3的linux符号表的小工具喵，小猫怕你觉得麻烦于是把符号表叼给你啦

![image-20260507212825558](https://raw.githubusercontent.com/lQ-A-Ql/blog-image/main/image-20260507212825558.png)

## 支持范围

MVP 支持：

- 远程 ISF：读取 `%USERPROFILE%\.meow\symbol-sources.txt`，默认查询 Abyss-W4tcher 开源符号库
- Ubuntu 18.04 / 20.04 / 22.04 / 24.04：远程 ISF 优先；未命中则自动解析并探测 `ddebs.ubuntu.com`
- Debian stable / oldstable 常见 amd64 kernel：远程 ISF 优先；未命中则自动解析并生成 `linux-image-<release>-dbg_<pkgver>_<arch>.deb` 候选
- RHEL / CentOS / Rocky / Alma / Fedora / openSUSE 常见服务器 banner：远程 ISF 优先；未命中则支持 `--repo-url`、`--vmlinux` 或手工 `.rpm` debug package 构建
- amd64 / x86_64
- WSL 后端
- 输入方式：banner、banner 文件、memdump、本地 `.ddeb/.deb/.rpm`、本地 vmlinux、手工 kernel/pkgver

暂不支持：

- Native Windows 完整后端
- 内置推断闭源/订阅 RPM 仓库地址
- 自编译 kernel、Android kernel、嵌入式 kernel
- ARM / MIPS / PowerPC
- GUI、上传功能
- 没有 debug symbol 或 vmlinux 时的符号恢复

## WSL 依赖

WSL 内需要：

```bash
sudo apt update
sudo apt install -y dpkg tar xz-utils curl rpm2cpio cpio gzip zstd
git clone https://github.com/volatilityfoundation/dwarf2json
cd dwarf2json
go build -o dwarf2json
sudo cp dwarf2json /usr/local/bin/
```

检查环境：

```powershell
.\meow.exe doctor
```

## 快速开始

```powershell
.\meow.exe doctor
.\meow.exe parse
.\meow.exe build --backend wsl --out .\symbols\linux
.\meow.exe verify --mem .\memdump.mem --symbols .\symbols
```

`parse` 和默认 `build` 会在终端提示粘贴 Linux kernel banner，粘贴后按 Enter。

## 解析 banner

```powershell
.\meow.exe parse
```

JSON 输出可配合管道：

```powershell
Get-Clipboard | .\meow.exe --json parse
```

远程符号源默认启用。`parse --json` 会显示 `symbol_sources_path`、`symbol_sources`、`remote_symbol_candidates`、`support_level`。

## 从 banner 生成

```powershell
.\meow.exe build --backend wsl --out .\symbols\linux
```

命令会提示在终端粘贴 Linux kernel banner，粘贴后按 Enter。

构建流程先查远程 ISF。若远程符号库已有匹配的 `.json.xz`，会直接下载到 `symbols/linux/`，不再下载 debug package。远程未命中时，Ubuntu/Debian 继续走 debug package 自动候选。

ddeb/deb/rpm 包通常很大，慢网络可拉长下载超时：

```powershell
.\meow.exe build --backend wsl --download-timeout 2h --out .\symbols\linux
```

如果卡在 “探测包” 阶段，可拉长探测总超时：

```powershell
.\meow.exe build --backend wsl --probe-timeout 2m --download-timeout 2h --out .\symbols\linux
```

普通模式会在下载时显示实际百分比；进入 WSL 后，会用整体构建进度显示解包、`dwarf2json` 与压缩阶段。解包时会额外显示第二条进度，按 debug package 内文件计数展示当前文件解包进度。进度条尖端是短稳定的 ASCII 像素小猫；JSON 模式不显示进度，保证 stdout 是纯 JSON。

```text
[*] 探测包        [==========^..^=__/         ] 1/3 linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
[*] 下载中         [==========^..^=__/         ]  39.4% 356.2 MB / 904.1 MB
[*] 构建符号       [==^..^=__/                       ]   8.0% 解包调试包
    解包文件       [=============^..^=__/            ] 24/61 ./usr/lib/debug/boot/vmlinux-5.4.0-163-generic
[*] 构建符号       [======^..^=__\                  ]  22.0% 运行 dwarf2json
[*] 构建符号       [========================^..^=__~]  97.0% 压缩 ISF
```

只解析、不下载、不生成：

```powershell
.\meow.exe build --dry-run
```

JSON 输出：

```powershell
.\meow.exe --json build --dry-run
```

禁用远程 ISF：

```powershell
.\meow.exe build --no-remote-symbols --dry-run
```

指定符号源 TXT：

```powershell
.\meow.exe build --symbol-sources C:\path\symbol-sources.txt
```

## 从内存镜像生成

需要本机可执行 `vol`：

```powershell
.\meow.exe build --mem .\memdump.mem --backend wsl --out .\symbols\linux
```

工具会调用：

```text
vol -f memdump.mem banners.Banners
```

然后提取 Linux banner 并进入自动构建流程。

## 手工指定参数

```powershell
.\meow.exe build `
  --distro ubuntu `
  --kernel 5.4.0-163-generic `
  --pkgver 5.4.0-163.180 `
  --arch amd64 `
  --backend wsl
```

## Debian / 服务器发行版

```powershell
.\meow.exe parse
.\meow.exe build --dry-run
```

Debian banner 会生成类似候选：

```text
https://deb.debian.org/debian/pool/main/l/linux/linux-image-5.10.0-35-amd64-dbg_5.10.237-1_amd64.deb
```

Debian 仓库会滚动，旧安全内核可能已从当前 pool 移走。遇到 404 时，使用 `snapshot.debian.org` 找到对应 `.deb` 后传：

```powershell
.\meow.exe build --ddeb-url <debian-debug-package-url> --backend wsl
```

RHEL/CentOS/Rocky/Alma/Fedora/openSUSE 当前优先查远程 ISF，不内置闭源或订阅仓库。若你有公开或内网 RPM repo base，可传 `--repo-url`，工具会读取 `repodata/repomd.xml` 和 primary metadata 精确找 `kernel-debuginfo`：

```powershell
.\meow.exe build --repo-url https://mirror.example.org/debug/os/x86_64/ --backend wsl
```

远程未命中且无 repo 时，可从目标系统或发行版仓库取得 `vmlinux`：

```powershell
.\meow.exe build --vmlinux .\vmlinux-4.18.0-513.5.1.el8_9.x86_64 --distro rhel --out .\symbols\linux
```

也可提供本地 RPM debuginfo：

```powershell
.\meow.exe build --debug-package .\kernel-debuginfo-4.18.0-513.5.1.el8_9.x86_64.rpm --backend wsl
```

## 从本地 debug package 生成

```powershell
.\meow.exe build --debug-package .\linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb --backend wsl
.\meow.exe build --debug-package .\linux-image-5.10.0-35-amd64-dbg_5.10.237-1_amd64.deb --backend wsl
.\meow.exe build --debug-package .\kernel-debuginfo-4.18.0-513.5.1.el8_9.x86_64.rpm --backend wsl
```

如果文件名无法推断 kernel/pkgver，补充：

```powershell
.\meow.exe build --debug-package .\kernel-debug-package.deb --kernel 5.10.0-35-amd64 --pkgver 5.10.237-1 --arch amd64 --distro debian
```

`--ddeb` / `--ddeb-url` 仍保留为兼容别名；新命令建议使用 `--debug-package` / `--debug-package-url`。

## 从本地 vmlinux 生成

```powershell
.\meow.exe build --vmlinux .\vmlinux-5.4.0-163-generic --out .\symbols\linux
```

这个模式不访问网络。

## 缓存

查看缓存目录：

```powershell
.\meow.exe cache path
```

列出下载缓存：

```powershell
.\meow.exe cache list
```

清理缓存：

```powershell
.\meow.exe cache clear
```

## 配置

查看默认配置：

```powershell
.\meow.exe config show
```

创建配置文件：

```powershell
.\meow.exe config init
```

默认路径：

```text
%USERPROFILE%\.meow\config.json
%USERPROFILE%\.meow\symbol-sources.txt
```

`%USERPROFILE%\.meow` 是当前默认配置目录。若你之前使用过旧版 `%USERPROFILE%\.volsym`，需要手工复制 `config.json`、`symbol-sources.txt` 或缓存文件到 `.meow`。

`config init` 会同时写出 `config.json` 和 `symbol-sources.txt`。符号源 TXT 一行一个源，支持 `#` 注释和空行：

```text
# name|index_url|raw_base_url
abyss|https://raw.githubusercontent.com/Abyss-W4tcher/volatility3-symbols/master/banners/banners_plain.json|https://raw.githubusercontent.com/Abyss-W4tcher/volatility3-symbols/master/
```

## 给 Volatility 3 使用

生成结果应位于：

```text
symbols/linux/Ubuntu_5.4.0-163-generic_5.4.0-163.180_amd64.json.xz
```

执行：

```powershell
vol -f .\memdump.mem -s .\symbols linux.pslist.PsList
```

注意 `-s` 指向 `symbols` 父目录，不是 `symbols/linux`。

## 常见错误

### 未找到 debug package

可能原因：

- banner 对应发行版当前不支持自动定位。
- debug package 被仓库清理或迁移。
- 包版本解析结果不正确。

处理：

```powershell
.\meow.exe build --ddeb-url <url> ...
.\meow.exe build --ddeb .\local.ddeb ...
.\meow.exe build --vmlinux .\vmlinux-...
```

### WSL 不可用

执行：

```powershell
wsl --install
.\meow.exe doctor
```

### 未找到 dwarf2json

在 WSL 内安装并复制到 PATH：

```bash
git clone https://github.com/volatilityfoundation/dwarf2json
cd dwarf2json
go build -o dwarf2json
sudo cp dwarf2json /usr/local/bin/
```

### Volatility 3 无法加载符号

检查：

- `-s` 是否指向 `symbols` 父目录。
- `.json.xz` 是否在 `symbols/linux/` 下。
- banner 是否与内存镜像匹配。
- 是否需要清理 Volatility 3 缓存。

