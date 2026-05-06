# Vol3 Linux Symbol Builder CLI PRD

## 1. 产品名称

**Vol3 Linux Symbol Builder**

简称：`meow`

一句话定位：

> 一个面向 Windows 用户的 Volatility 3 Linux 符号表自动生成 CLI 工具。输入内核 banner、内核版本或内存镜像，自动定位调试符号仓库，下载对应 debug symbol 包，提取 `vmlinux`，调用 `dwarf2json`，生成 Volatility 3 可用的 `.json.xz` 符号表。

---

## 2. 背景

Volatility 3 分析 Linux 内存镜像时，经常因为缺少精确匹配的符号表而无法运行 `linux.pslist.PsList`、`linux.lsmod.Lsmod`、`linux.bash.Bash` 等插件。

当前用户通常需要手工完成以下流程：

1. 从 `banners.Banners` 中复制 Linux kernel banner。
2. 人工判断发行版、内核版本、架构、包版本。
3. 手工搜索 Ubuntu / Debian / CentOS / Fedora 等发行版的 debug symbol 仓库。
4. 下载 `.ddeb`、`.debuginfo.rpm` 或其他调试包。
5. 解包找到 `vmlinux`。
6. 编译或安装 `dwarf2json`。
7. 调用 `dwarf2json linux --elf vmlinux` 生成 ISF JSON。
8. 压缩为 `.json.xz`。
9. 放入 Volatility 3 的 `symbols/linux/` 目录。
10. 清理 Volatility 3 缓存后重新验证。

这个流程低效、重复、容易出错，尤其对 Windows 用户极不友好。

本产品的目标就是把这条链路做成可靠、可复现、可批量执行的 CLI 工具。

---

## 3. 问题定义

### 3.1 现在的痛点

#### 痛点一：用户不知道去哪找符号包

用户拿到：

```text
Linux version 5.4.0-163-generic ... (Ubuntu 5.4.0-163.180-generic 5.4.246)
```

但不知道应该去：

```text
http://ddebs.ubuntu.com/pool/main/l/linux/
```

更不知道目标文件名应该是：

```text
linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
```

这不是用户懒，是这套规则本身就反人类。

#### 痛点二：只看 `uname -r` 会出错

`5.4.0-163-generic` 不等于完整包版本。

真正需要的是：

```text
uname_r = 5.4.0-163-generic
pkgver  = 5.4.0-163.180
arch    = amd64
```

只靠 `5.4.0-163-generic` 盲猜，迟早下载错包。

#### 痛点三：Windows 用户环境割裂

很多用户在 Windows 上做 CTF / DFIR，但符号生成流程更偏 Linux：

- `dpkg-deb`
- `xz`
- `wget`
- `dwarf2json`
- Volatility 3 Python 环境

让用户手动在 Windows 和 WSL 之间来回切换，是一种产品失败。

#### 痛点四：错误信息不透明

现在失败时常见的用户体验是：

```text
Unsatisfied requirement plugins.PsList.kernel.symbol_table_name
```

这句话对人没有帮助。

用户真正需要知道的是：

```text
缺少 Linux 5.4.0-163-generic / Ubuntu 5.4.0-163.180 / amd64 的符号表。
建议下载 linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb。
```

---

## 4. 产品目标

### 4.1 核心目标

MVP 阶段必须做到：

1. 用户输入 Linux banner，工具自动解析出发行版、内核版本、包版本和架构。
2. 用户输入内存镜像，工具可以调用 Volatility 3 的 `banners.Banners` 自动提取 banner。
3. 对 Ubuntu 系内核，工具能够自动定位 `ddebs.ubuntu.com` 的 debug symbol 包。
4. 工具能够通过 WSL 后端完成下载、解包、生成、压缩全过程。
5. 工具输出 Volatility 3 可直接使用的 `.json.xz`。
6. 工具能够对生成结果做基本校验。
7. 工具必须给出可读错误信息，而不是把底层报错原样甩给用户。

### 4.2 非常明确的产品原则

#### 原则一：不要让用户猜

用户不应该猜：

- 仓库在哪。
- 包名叫什么。
- `vmlinux` 在哪。
- 符号表应该放哪。
- 报错是什么意思。

工具猜，工具验证，工具失败时告诉用户下一步。

#### 原则二：不要假装支持一切

MVP 只稳定支持 Ubuntu。

不允许在 README 写“支持 Linux”，然后实际上只在一个 Ubuntu 版本上能跑。

文档必须写清楚：

```text
MVP 支持：Ubuntu 18.04 / 20.04 / 22.04 / 24.04 的 amd64 generic kernel。
实验支持：Debian。
暂不支持：自编译内核、裁剪发行版、Android kernel、嵌入式 kernel。
```

#### 原则三：失败要失败得明白

错误信息必须说明：

1. 当前执行到哪一步。
2. 为什么失败。
3. 用户可以怎么修。
4. 是否能手工指定参数绕过。

不接受这种错误：

```text
failed
```

必须是这种错误：

```text
[ERROR] 未找到对应 ddeb 包。
Kernel: 5.4.0-163-generic
Package Version: 5.4.0-163.180
Arch: amd64
Tried:
  - http://ddebs.ubuntu.com/pool/main/l/linux/linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
  - http://ddebs.ubuntu.com/pool/main/l/linux/linux-image-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
建议：使用 --ddeb-url 手工指定下载地址，或确认 banner 是否来自 Ubuntu 官方内核。
```

---

## 5. 目标用户

### 5.1 CTF 选手

典型需求：

- 拿到 Linux 内存镜像。
- 跑 Volatility 3。
- 因为缺符号表卡住。
- 希望快速生成符号表继续做题。

### 5.2 应急响应 / DFIR 分析人员

典型需求：

- 分析客户 Linux 服务器内存。
- 需要快速还原进程、模块、命令历史、网络连接等信息。
- 不希望把时间浪费在找符号表上。

### 5.3 安全研究人员

典型需求：

- 批量分析不同 Linux kernel 的内存镜像。
- 希望符号生成过程可缓存、可复现、可自动化。

---

## 6. 使用场景

### 6.1 场景一：用户已有 banner

输入：

```bash
meow build --banner "Linux version 5.4.0-163-generic ... (Ubuntu 5.4.0-163.180-generic 5.4.246)" --backend wsl
```

输出：

```text
[+] Distro: Ubuntu
[+] Release: focal / 20.04
[+] Kernel: 5.4.0-163-generic
[+] Package Version: 5.4.0-163.180
[+] Arch: amd64
[+] Found ddeb: linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
[+] Downloaded
[+] Extracted vmlinux
[+] Generated ISF
[+] Compressed json.xz
[+] Output: ./symbols/linux/Ubuntu_5.4.0-163-generic_5.4.0-163.180_amd64.json.xz
```

### 6.2 场景二：用户只有内存镜像

输入：

```bash
meow build --mem ./memdump.mem --backend wsl
```

工具行为：

1. 调用 Volatility 3：

```bash
vol -f ./memdump.mem banners.Banners
```

2. 提取 banner。
3. 走自动生成流程。

### 6.3 场景三：自动解析失败，用户手工指定

输入：

```bash
meow build \
  --kernel 5.4.0-163-generic \
  --pkgver 5.4.0-163.180 \
  --distro ubuntu \
  --arch amd64 \
  --backend wsl
```

工具不应该因为 banner 解析失败就直接死掉。

用户愿意手填参数时，必须允许继续。

### 6.4 场景四：用户已有 ddeb

输入：

```bash
meow build --ddeb ./linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb --backend wsl
```

工具行为：

1. 不下载。
2. 直接解包。
3. 自动查找 `vmlinux-*`。
4. 生成符号表。

### 6.5 场景五：用户已有 vmlinux

输入：

```bash
meow build --vmlinux ./vmlinux-5.4.0-163-generic
```

工具行为：

1. 直接调用 `dwarf2json`。
2. 生成 `.json.xz`。
3. 可选校验 banner。

---

## 7. MVP 范围

### 7.1 必须支持

MVP 必须支持以下能力：

| 功能 | 是否必须 |
|---|---|
| 解析 Ubuntu banner | 必须 |
| 根据 Ubuntu banner 生成 ddeb 候选 URL | 必须 |
| 通过 HTTP HEAD 判断 ddeb 是否存在 | 必须 |
| 下载 ddeb | 必须 |
| 缓存已下载 ddeb | 必须 |
| WSL 后端执行解包 | 必须 |
| WSL 后端调用 dwarf2json | 必须 |
| 生成 `.json.xz` | 必须 |
| 输出到指定目录 | 必须 |
| 错误信息结构化 | 必须 |
| `--ddeb` 本地文件输入 | 必须 |
| `--vmlinux` 本地文件输入 | 必须 |
| `--dry-run` 只解析不下载 | 必须 |
| `--verbose` 详细日志 | 必须 |
| `--force` 强制重新生成 | 必须 |

### 7.2 暂不支持

MVP 不支持：

1. 自动适配所有 Linux 发行版。
2. 自动生成自编译 kernel 的符号表。
3. Android kernel。
4. ARM / MIPS / PowerPC 架构。
5. 图形界面。
6. 在线符号库服务。
7. 自动上传符号表。
8. 自动破解或绕过缺失 debug symbol 的情况。

话说清楚：**没有 debug symbol 或 vmlinux，就不可能凭空生成准确 Volatility 3 Linux 符号表。**

---

## 8. 后续版本范围

### 8.1 V1.1

增加：

- Debian 支持。
- `apt-file` / snapshot.debian.org 检索。
- 更多 Ubuntu kernel flavour：
  - `generic`
  - `lowlatency`
  - `aws`
  - `azure`
  - `gcp`
  - `oracle`
  - `kvm`

### 8.2 V1.2

增加：

- Windows 原生后端。
- 内置 `dwarf2json.exe`。
- 内置 `7z.exe` 或原生解包实现。
- 不依赖 WSL 的完整流程。

### 8.3 V1.3

增加：

- 批量生成。
- 符号表索引库。
- 本地符号表缓存查询。
- `meow list` 查看已生成符号。

### 8.4 V2.0

增加：

- GUI。
- 一键拖入内存镜像生成符号。
- 自动调用 Volatility 3 验证插件。
- 任务队列与进度展示。

---

## 9. 核心命令设计

### 9.1 根命令

```bash
meow [command]
```

### 9.2 命令列表

```bash
meow parse
meow build
meow verify
meow cache
meow config
meow doctor
```

---

## 10. `parse` 命令

### 10.1 功能

解析 banner，不下载，不生成。

### 10.2 输入

```bash
meow parse --banner "Linux version ..."
meow parse --banner-file ./banner.txt
```

### 10.3 输出

```text
Distro          Ubuntu
Codename        focal
Kernel          5.4.0-163-generic
Package Version 5.4.0-163.180
Arch            amd64
Source Package  linux
Repo Base        http://ddebs.ubuntu.com/pool/main/l/linux/
Candidate ddeb   linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
```

### 10.4 JSON 输出

```bash
meow parse --banner-file ./banner.txt --json
```

```json
{
  "distro": "ubuntu",
  "codename": "focal",
  "kernel": "5.4.0-163-generic",
  "package_version": "5.4.0-163.180",
  "arch": "amd64",
  "source_package": "linux",
  "repo_base": "http://ddebs.ubuntu.com/pool/main/l/linux/",
  "candidates": [
    "http://ddebs.ubuntu.com/pool/main/l/linux/linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb",
    "http://ddebs.ubuntu.com/pool/main/l/linux/linux-image-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb"
  ]
}
```

---

## 11. `build` 命令

### 11.1 功能

生成 Volatility 3 Linux ISF 符号表。

### 11.2 输入方式

支持以下输入优先级：

1. `--vmlinux`
2. `--ddeb`
3. `--banner` / `--banner-file`
4. `--mem`
5. `--kernel + --pkgver + --distro + --arch`

### 11.3 命令示例

#### 从 banner 构建

```bash
meow build --banner-file ./banner.txt --backend wsl --out ./symbols/linux
```

#### 从内存镜像构建

```bash
meow build --mem ./memdump.mem --backend wsl --out ./symbols/linux
```

#### 手工指定参数构建

```bash
meow build \
  --distro ubuntu \
  --kernel 5.4.0-163-generic \
  --pkgver 5.4.0-163.180 \
  --arch amd64 \
  --backend wsl
```

#### 从本地 ddeb 构建

```bash
meow build --ddeb ./kernel-dbgsym.ddeb --backend wsl
```

#### 从本地 vmlinux 构建

```bash
meow build --vmlinux ./vmlinux-5.4.0-163-generic
```

### 11.4 关键参数

| 参数 | 说明 | 必须 |
|---|---|---|
| `--banner` | 直接传入 banner 字符串 | 否 |
| `--banner-file` | 从文件读取 banner | 否 |
| `--mem` | 内存镜像路径 | 否 |
| `--kernel` | kernel release，例如 `5.4.0-163-generic` | 否 |
| `--pkgver` | 包版本，例如 `5.4.0-163.180` | 否 |
| `--distro` | 发行版，例如 `ubuntu` | 否 |
| `--arch` | 架构，默认 `amd64` | 否 |
| `--backend` | 后端：`wsl` / `native` | 是 |
| `--out` | 输出目录 | 否 |
| `--cache-dir` | 缓存目录 | 否 |
| `--force` | 强制重新下载/生成 | 否 |
| `--dry-run` | 只解析和检查，不执行下载生成 | 否 |
| `--verbose` | 输出详细日志 | 否 |
| `--json` | 以 JSON 输出结果 | 否 |

---

## 12. `verify` 命令

### 12.1 功能

验证生成的符号表是否可被 Volatility 3 使用。

### 12.2 命令示例

```bash
meow verify --mem ./memdump.mem --symbols ./symbols/linux
```

### 12.3 验证动作

工具需要执行：

```bash
vol -f ./memdump.mem -s ./symbols linux.banners.Banners
vol -f ./memdump.mem -s ./symbols linux.pslist.PsList
```

### 12.4 输出

成功：

```text
[+] Volatility 3 loaded symbol table successfully.
[+] linux.pslist.PsList executed successfully.
```

失败：

```text
[ERROR] 符号表未被 Volatility 3 加载。
Possible causes:
  1. symbols/linux 目录层级错误。
  2. json.xz 文件损坏。
  3. banner 不匹配。
  4. Volatility 3 缓存未清理。
建议执行：meow cache clear
```

---

## 13. `cache` 命令

### 13.1 功能

管理下载包、解包文件和生成结果缓存。

### 13.2 子命令

```bash
meow cache list
meow cache clear
meow cache path
```

### 13.3 缓存内容

```text
cache/
├── downloads/
│   └── *.ddeb
├── extracted/
│   └── */vmlinux-*
├── json/
│   └── *.json
└── symbols/
    └── *.json.xz
```

---

## 14. `doctor` 命令

### 14.1 功能

检查环境依赖。

### 14.2 检查项

| 检查项 | WSL 后端 | Native 后端 |
|---|---|---|
| WSL 是否安装 | 必须 | 不需要 |
| WSL 发行版是否可用 | 必须 | 不需要 |
| `bash` | 必须 | 不需要 |
| `dpkg-deb` | 必须 | 不需要 |
| `xz` | 必须 | 可选 |
| `wget` / `curl` | 必须 | 可选 |
| `dwarf2json` | 必须 | 必须 |
| `vol` / `vol.py` | 验证时必须 | 验证时必须 |
| `7z.exe` | 不需要 | 必须 |

### 14.3 输出示例

```text
[+] OS: Windows 10 x64
[+] WSL: Installed
[+] WSL Distro: Ubuntu-22.04
[+] dpkg-deb: OK
[+] xz: OK
[+] dwarf2json: OK
[!] Volatility 3: Not found

Result: build available, verify unavailable.
```

---

## 15. Ubuntu 仓库定位规则

### 15.1 输入样例

```text
Linux version 5.4.0-163-generic (buildd@lcy02-amd64-067) (gcc version 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.2)) #180-Ubuntu SMP Tue Sep 5 13:21:23 UTC 2023 (Ubuntu 5.4.0-163.180-generic 5.4.246)
```

### 15.2 解析结果

```text
distro = ubuntu
codename = focal
kernel = 5.4.0-163-generic
pkgver = 5.4.0-163.180
arch = amd64
source_package = linux
```

### 15.3 包名候选规则

优先级从高到低：

```text
linux-image-unsigned-${kernel}-dbgsym_${pkgver}_${arch}.ddeb
linux-image-${kernel}-dbgsym_${pkgver}_${arch}.ddeb
linux-modules-${kernel}-dbgsym_${pkgver}_${arch}.ddeb
```

MVP 阶段主要使用前两个。

### 15.4 仓库 URL 规则

```text
http://ddebs.ubuntu.com/pool/main/l/linux/${package_name}
```

### 15.5 完整示例

```text
http://ddebs.ubuntu.com/pool/main/l/linux/linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
```

---

## 16. Banner 解析规则

### 16.1 提取 kernel release

正则：

```regex
Linux version\s+([^\s]+)
```

样例结果：

```text
5.4.0-163-generic
```

### 16.2 提取 Ubuntu package version

正则：

```regex
Ubuntu\s+([0-9]+\.[0-9]+\.[0-9]+-[0-9]+\.[0-9]+)
```

样例结果：

```text
5.4.0-163.180
```

### 16.3 推断 Ubuntu codename

规则：

| 特征 | codename |
|---|---|
| `~18.04` | bionic |
| `~20.04` | focal |
| `~22.04` | jammy |
| `~24.04` | noble |

如果 banner 中无法推断 codename，MVP 可以不阻塞，因为 `pool/main/l/linux/` 不强依赖 codename。

### 16.4 架构判断

默认：

```text
amd64
```

如果 banner 包含：

```text
x86_64
```

则映射为：

```text
amd64
```

如果用户手动传入 `--arch`，以用户传入为准。

---

## 17. 后端设计

### 17.1 WSL 后端

MVP 默认后端。

#### 17.1.1 职责

WSL 后端负责：

1. 下载或接收 ddeb 文件。
2. 执行 `dpkg-deb -x`。
3. 查找 `vmlinux-*`。
4. 执行 `dwarf2json`。
5. 执行 `xz` 压缩。
6. 输出符号表到 Windows 可访问路径。

#### 17.1.2 Windows 到 WSL 路径转换

工具必须支持路径转换：

```text
C:\Users\QAQ\symbols
```

转换为：

```text
/mnt/c/Users/QAQ/symbols
```

不允许让用户自己手动写 `/mnt/c/...`。

#### 17.1.3 WSL 调用方式

```text
wsl.exe -d <distro> bash -lc "<command>"
```

如果用户没有指定 distro，使用默认 WSL 发行版。

### 17.2 Native 后端

V1.2 支持。

#### 17.2.1 职责

Native 后端负责：

1. Windows 原生下载 ddeb。
2. 调用 `7z.exe` 解包。
3. 提取 `data.tar.*`。
4. 解包出 `vmlinux-*`。
5. 调用 `dwarf2json.exe`。
6. 调用 `xz.exe` 或内部实现压缩。

#### 17.2.2 原生依赖

```text
tools/
├── dwarf2json.exe
├── 7z.exe
├── 7z.dll
└── xz.exe
```

---

## 18. 输出文件命名规范

符号表输出文件名必须包含足够信息。

格式：

```text
${Distro}_${Kernel}_${PackageVersion}_${Arch}.json.xz
```

示例：

```text
Ubuntu_5.4.0-163-generic_5.4.0-163.180_amd64.json.xz
```

不接受这种命名：

```text
symbol.json.xz
linux.json.xz
final.json.xz
```

这种命名对排错毫无帮助。

---

## 19. 日志规范

### 19.1 普通日志

普通模式输出必要进度：

```text
[+] Parsing banner
[+] Resolving Ubuntu debug symbol package
[+] Downloading ddeb
[+] Extracting vmlinux
[+] Running dwarf2json
[+] Compressing ISF
[+] Done
```

### 19.2 详细日志

`--verbose` 输出：

1. 请求 URL。
2. HTTP 状态码。
3. 文件大小。
4. 缓存命中情况。
5. WSL 命令。
6. `dwarf2json` 执行时间。
7. 输出文件 hash。

### 19.3 日志等级

```text
[+] 成功步骤
[*] 普通信息
[!] 警告
[-] 非致命失败
[ERROR] 致命错误
```

---

## 20. 错误处理要求

### 20.1 Banner 解析失败

错误信息：

```text
[ERROR] 无法从 banner 中提取 Ubuntu package version。
已提取 kernel: 5.4.0-163-generic
缺失字段: package_version
建议：手工指定 --pkgver，例如 --pkgver 5.4.0-163.180
```

### 20.2 ddeb 不存在

错误信息：

```text
[ERROR] 未找到对应 debug symbol 包。
Tried URLs:
  - ...
  - ...
建议：
  1. 检查 kernel 是否来自 Ubuntu 官方内核。
  2. 使用 --ddeb-url 手工指定。
  3. 使用 --vmlinux 直接生成。
```

### 20.3 WSL 不可用

错误信息：

```text
[ERROR] WSL 后端不可用。
原因：wsl.exe 执行失败。
建议：
  1. 安装 WSL。
  2. 安装 Ubuntu 子系统。
  3. 或使用 --backend native。
```

### 20.4 dwarf2json 不存在

错误信息：

```text
[ERROR] 未找到 dwarf2json。
WSL 中请执行：
  git clone https://github.com/volatilityfoundation/dwarf2json
  cd dwarf2json
  go build -o dwarf2json
  sudo cp dwarf2json /usr/local/bin/
```

后续版本应提供自动安装选项，但 MVP 不强制。

### 20.5 未找到 vmlinux

错误信息：

```text
[ERROR] ddeb 已解包，但未找到 vmlinux。
Search path:
  dbgsym/usr/lib/debug/boot/
Found candidates:
  <列出 find 结果>
建议：确认 ddeb 是否为 linux-image dbgsym 包，而不是 modules dbgsym 包。
```

---

## 21. 配置文件

### 21.1 默认路径

Windows：

```text
%USERPROFILE%\.meow\config.json
```

Linux / WSL：

```text
~/.meow/config.json
```

### 21.2 配置示例

```json
{
  "backend": "wsl",
  "wsl_distro": "Ubuntu-22.04",
  "cache_dir": "C:\\Users\\QAQ\\.meow\\cache",
  "output_dir": "C:\\Users\\QAQ\\Vol3Symbols\\linux",
  "volatility_path": "vol",
  "auto_clear_vol_cache": false,
  "download_timeout_seconds": 60,
  "max_retries": 3
}
```

---

## 22. 缓存策略

### 22.1 下载缓存

同一个 ddeb 不重复下载。

缓存 key：

```text
sha256(url)
```

文件名保留原始包名。

### 22.2 生成缓存

如果目标 `.json.xz` 已存在，默认不重新生成。

用户可以用：

```bash
meow build --force
```

强制重新生成。

### 22.3 缓存校验

下载完成后记录：

```json
{
  "url": "...",
  "filename": "...",
  "size": 927000000,
  "sha256": "...",
  "downloaded_at": "..."
}
```

---

## 23. 安全要求

### 23.1 不执行不可信二进制

工具只允许：

- 解包 ddeb。
- 读取 ELF/DWARF。
- 调用 `dwarf2json`。

不允许执行解包出来的任何文件。

### 23.2 下载来源透明

每次下载前必须显示 URL。

`--json` 模式也必须包含 URL。

### 23.3 不默认上传任何文件

工具不允许上传：

- 内存镜像。
- vmlinux。
- 符号表。
- 用户日志。

除非未来版本明确增加远程服务，并且默认关闭。

### 23.4 命令注入防护

所有传入 WSL 的参数必须做转义。

尤其是：

- 路径。
- URL。
- banner 字符串。
- 输出文件名。

不允许直接拼接未转义字符串执行 shell。

---

## 24. 性能要求

### 24.1 下载

- 支持断点续传：V1.1。
- MVP 至少要支持缓存，避免重复下载大文件。

### 24.2 dwarf2json 执行

- 对 500MB 到 2GB 级别的 vmlinux 处理不能无提示卡死。
- 必须持续输出进度或至少输出当前阶段。

### 24.3 内存占用

- 生成 JSON 时允许较高内存占用，但工具本身不要把大文件一次性读入内存。
- 下载、解包、压缩均使用流式或外部命令。

---

## 25. 验收标准

### 25.1 Ubuntu 20.04 验收用例

输入 banner：

```text
Linux version 5.4.0-163-generic (buildd@lcy02-amd64-067) (gcc version 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.2)) #180-Ubuntu SMP Tue Sep 5 13:21:23 UTC 2023 (Ubuntu 5.4.0-163.180-generic 5.4.246)
```

执行：

```bash
meow build --banner-file ./banner.txt --backend wsl --out ./symbols/linux
```

必须生成：

```text
Ubuntu_5.4.0-163-generic_5.4.0-163.180_amd64.json.xz
```

必须尝试 URL：

```text
http://ddebs.ubuntu.com/pool/main/l/linux/linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
```

必须从 ddeb 中找到：

```text
usr/lib/debug/boot/vmlinux-5.4.0-163-generic
```

### 25.2 dry-run 验收

执行：

```bash
meow build --banner-file ./banner.txt --dry-run
```

不得下载文件。

必须输出：

- 解析结果。
- 候选 URL。
- 输出文件名。

### 25.3 本地 vmlinux 验收

执行：

```bash
meow build --vmlinux ./vmlinux-5.4.0-163-generic --out ./symbols/linux
```

不得访问网络。

必须生成 `.json.xz`。

### 25.4 错误信息验收

断网时执行下载。

不允许只输出：

```text
connection failed
```

必须输出：

- 当前下载 URL。
- 网络错误原因。
- 是否已使用缓存。
- 建议手动下载或重试。

---

## 26. 技术选型

### 26.1 CLI 语言

推荐：Go。

理由：

1. 单文件 Windows exe，分发省事。
2. 调用外部命令简单。
3. 下载、并发、日志都方便。
4. `dwarf2json` 本身也是 Go 项目，生态一致。
5. 后续跨平台成本低。

### 26.2 CLI 框架

推荐：

```text
cobra
```

### 26.3 输出美化

推荐：

```text
pterm
```

或者保持零依赖，自己实现简单日志。

### 26.4 配置解析

推荐：

```text
encoding/json
```

MVP 不引入 YAML，减少依赖。

---

## 27. 代码结构建议

```text
meow/
├── main.go
├── cmd/
│   ├── root.go
│   ├── parse.go
│   ├── build.go
│   ├── verify.go
│   ├── cache.go
│   └── doctor.go
├── internal/
│   ├── banner/
│   │   └── ubuntu.go
│   ├── distro/
│   │   └── ubuntu.go
│   ├── resolver/
│   │   └── ddeb.go
│   ├── downloader/
│   │   └── http.go
│   ├── backend/
│   │   ├── wsl.go
│   │   └── native.go
│   ├── runner/
│   │   └── command.go
│   ├── cache/
│   │   └── cache.go
│   ├── symbols/
│   │   └── build.go
│   └── log/
│       └── log.go
├── scripts/
│   └── wsl_build.sh
├── testdata/
│   └── banners/
│       └── ubuntu_5.4.0_163.txt
└── README.md
```

---

## 28. 数据结构设计

### 28.1 KernelInfo

```go
 type KernelInfo struct {
     Distro         string
     Codename       string
     KernelRelease  string
     PackageVersion string
     Arch           string
     SourcePackage  string
     Banner         string
 }
```

### 28.2 ResolveResult

```go
 type ResolveResult struct {
     KernelInfo KernelInfo
     Candidates []string
     FoundURL   string
     PackageName string
 }
```

### 28.3 BuildResult

```go
 type BuildResult struct {
     SymbolPath string
     VmlinuxPath string
     PackagePath string
     DurationSeconds float64
     Success bool
 }
```

---

## 29. Repository Resolver 详细逻辑

### 29.1 Ubuntu resolver 输入

```go
KernelInfo{
    Distro: "ubuntu",
    KernelRelease: "5.4.0-163-generic",
    PackageVersion: "5.4.0-163.180",
    Arch: "amd64",
    SourcePackage: "linux",
}
```

### 29.2 输出候选 URL

```go
[]string{
    "http://ddebs.ubuntu.com/pool/main/l/linux/linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb",
    "http://ddebs.ubuntu.com/pool/main/l/linux/linux-image-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb",
}
```

### 29.3 判断存在

使用 HTTP HEAD。

如果 HEAD 被服务器拒绝，则 fallback 到 GET Range：

```text
Range: bytes=0-0
```

不要因为 HEAD 不通就判定不存在。

---

## 30. WSL 脚本设计

### 30.1 输入参数

```bash
wsl_build.sh \
  --ddeb /mnt/c/path/to/package.ddeb \
  --kernel 5.4.0-163-generic \
  --pkgver 5.4.0-163.180 \
  --arch amd64 \
  --out /mnt/c/path/to/symbols/linux
```

### 30.2 脚本行为

1. 检查 `dpkg-deb`。
2. 检查 `xz`。
3. 检查 `dwarf2json`。
4. 解包 ddeb。
5. 查找 `vmlinux-${kernel}`。
6. 调用 `dwarf2json`。
7. 压缩。
8. 移动输出。

### 30.3 脚本必须 set 严格模式

```bash
set -euo pipefail
```

脚本不能吞错误。

---

## 31. 用户体验要求

### 31.1 默认行为必须聪明

用户执行：

```bash
meow build --banner-file banner.txt
```

工具应默认：

- 使用 WSL 后端。
- 输出到当前目录下 `symbols/linux/`。
- 使用缓存目录。
- 不覆盖已有符号表。

### 31.2 不要强迫用户提供已知信息

banner 里能解析出来的，不要要求用户手填。

### 31.3 输出必须适合复制到报告

最终结果输出应包含：

```text
Kernel: 5.4.0-163-generic
Package: 5.4.0-163.180
Distro: Ubuntu focal
Symbol: Ubuntu_5.4.0-163-generic_5.4.0-163.180_amd64.json.xz
```

---

## 32. README 必须包含的内容

README 不允许只写安装命令。

必须包含：

1. 这个工具解决什么问题。
2. 支持范围。
3. 不支持范围。
4. WSL 后端依赖。
5. 快速开始。
6. 从 banner 生成。
7. 从 memdump 生成。
8. 从 ddeb 生成。
9. 从 vmlinux 生成。
10. 常见错误。
11. 生成结果如何给 Volatility 3 使用。

---

## 33. 示例快速开始

```bash
meow doctor
meow parse --banner-file banner.txt
meow build --banner-file banner.txt --backend wsl
meow verify --mem memdump.mem --symbols ./symbols
```

---

## 34. 里程碑

### Milestone 1：Ubuntu banner 解析

交付内容：

- `meow parse`
- Ubuntu banner 正则解析
- ddeb URL 生成
- JSON 输出

验收：

- 能正确解析 `5.4.0-163-generic` 示例 banner。

### Milestone 2：WSL 后端打通

交付内容：

- `meow doctor`
- WSL 命令执行器
- 路径转换
- WSL 环境检查

验收：

- 能在 Windows 下调用 WSL 执行 `bash -lc`。

### Milestone 3：下载和缓存

交付内容：

- HTTP resolver
- 下载器
- 缓存目录
- 失败重试

验收：

- 能下载并缓存 Ubuntu ddeb。

### Milestone 4：符号生成

交付内容：

- ddeb 解包
- vmlinux 查找
- dwarf2json 调用
- xz 压缩

验收：

- 能生成 `.json.xz`。

### Milestone 5：验证和错误处理

交付内容：

- `meow verify`
- Volatility 3 调用
- 结构化错误信息

验收：

- 成功时能跑 `linux.pslist.PsList`。
- 失败时能明确指出原因。

---

## 35. 风险和应对

### 35.1 ddeb 包不存在

风险：

某些旧内核包可能被清理或迁移。

应对：

- 支持手工指定 `--ddeb-url`。
- 支持本地 `--ddeb`。
- 支持本地 `--vmlinux`。

### 35.2 banner 不完整

风险：

内存镜像中的 banner 被截断。

应对：

- 提供手工参数模式。
- 工具提示缺失字段。

### 35.3 WSL 环境混乱

风险：

用户 WSL 没装依赖。

应对：

- `meow doctor` 检查。
- 输出安装命令。
- 后续提供 `meow doctor --fix`。

### 35.4 dwarf2json 版本问题

风险：

不同版本 dwarf2json 输出不兼容。

应对：

- 记录 dwarf2json 版本。
- 在日志中输出路径和版本。
- 后续内置固定版本。

---

## 36. 明确不做的事情

以下事情不要塞进 MVP：

1. 不做 GUI。
2. 不做在线服务。
3. 不做自动上传。
4. 不做所有发行版支持。
5. 不做 kernel debug symbol 的魔法恢复。
6. 不做 Volatility 3 插件开发。
7. 不做内存镜像分析报告生成。

这个工具只解决一件事：

> 给 Volatility 3 生成能用的 Linux 符号表。

其它事情不要污染 MVP。

---

## 37. 成功指标

### 37.1 功能成功指标

- Ubuntu 官方 generic kernel 符号生成成功率 ≥ 90%。
- 已有 ddeb 输入场景成功率 ≥ 95%。
- 已有 vmlinux 输入场景成功率 ≥ 98%。

### 37.2 用户体验指标

- 从 banner 到生成符号表，用户只需一条命令。
- 常见错误必须给出可执行建议。
- 用户不需要手动进入 WSL。

### 37.3 工程指标

- Windows 单文件可运行。
- `meow doctor` 能在 10 秒内完成基础环境检查。
- 所有 resolver 逻辑必须有单元测试。

---

## 38. 最小可用版本定义

MVP 完成标准：

用户在 Windows 上执行：

```bash
meow build --banner-file banner.txt --backend wsl
```

工具能够自动生成：

```text
symbols/linux/Ubuntu_5.4.0-163-generic_5.4.0-163.180_amd64.json.xz
```

并且用户可以直接执行：

```bash
vol -f memdump.mem -s ./symbols linux.pslist.PsList
```

如果这条链路跑不通，这个产品就没有完成。

不要用“已经实现了解析模块”“已经能下载文件”来冒充完成。

产品的完成标准只有一个：

> 生成的符号表能让 Volatility 3 成功跑 Linux 插件。

---

## 39. 示例最终输出

```text
[+] Vol3 Linux Symbol Builder

[*] Parsing banner
    Distro          : Ubuntu
    Codename        : focal
    Kernel          : 5.4.0-163-generic
    Package Version : 5.4.0-163.180
    Arch            : amd64

[*] Resolving debug symbol package
    Candidate       : linux-image-unsigned-5.4.0-163-generic-dbgsym_5.4.0-163.180_amd64.ddeb
    Repo            : http://ddebs.ubuntu.com/pool/main/l/linux/

[+] Package found
[+] Download completed
[+] vmlinux extracted
[+] dwarf2json completed
[+] Compressed ISF

[+] Symbol generated successfully
    Output: ./symbols/linux/Ubuntu_5.4.0-163-generic_5.4.0-163.180_amd64.json.xz

Next:
    vol -f memdump.mem -s ./symbols linux.pslist.PsList
```

---

## 40. 总结

这个产品不追求炫技。

它要解决的是一个非常具体但非常烦人的问题：

> Windows 用户拿到 Linux 内存镜像后，不应该再被 Volatility 3 Linux 符号表卡死。

第一版不要贪。

先把 Ubuntu + WSL + ddeb + dwarf2json + `.json.xz` 这条链路做得稳定、清楚、可复现。

如果这条链路不稳，支持再多发行版都是假繁荣。

MVP 的判断标准很粗暴：

```text
输入 banner，输出可用符号表。
```

除此之外，都是锦上添花。


