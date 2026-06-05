export type PluginEntry = {
  name: string
  description: string
}

export type PluginCategory = {
  name: string
  icon: string
  plugins: PluginEntry[]
}

export const pluginCategories: PluginCategory[] = [
  {
    name: "Linux",
    icon: "🐧",
    plugins: [
      { name: "linux.pslist.PsList", description: "列出所有进程 (EPROCESS)" },
      { name: "linux.pstree.PsTree", description: "以树状结构显示进程关系" },
      { name: "linux.psscan.PsScan", description: "扫描隐藏/终止的进程" },
      { name: "linux.files.Files", description: "列出所有打开的文件描述符" },
      { name: "linux.proc_maps.ProcMaps", description: "显示进程内存映射" },
      { name: "linux.bash.Bash", description: "从 bash_history 恢复命令历史" },
      { name: "linux.lsmod.Lsmod", description: "列出已加载内核模块" },
      { name: "linux.lsof.Lsof", description: "列出所有进程打开的文件" },
      { name: "linux.sockstat.Sockstat", description: "显示网络套接字状态" },
      { name: "linux.tun.Tun", description: "检测 TUN/TAP 网络接口" },
      { name: "linux.tty_check.Tty_check", description: "检查 TTY 设备劫持" },
      { name: "linux.check_syscall.Check_syscall", description: "检查系统调用表挂钩" },
      { name: "linux.check_modules.Check_modules", description: "检测未列出的内核模块" },
      { name: "linux.malfind.Malfind", description: "检测进程注入和恶意代码" },
      { name: "linux.elfs.Elfs", description: "列出所有内存中的 ELF 文件" },
      { name: "linux.pagecache.Pagecache", description: "分析内核页面缓存" },
    ],
  },
  {
    name: "Windows",
    icon: "🪟",
    plugins: [
      { name: "windows.pslist.PsList", description: "列出所有进程" },
      { name: "windows.pstree.PsTree", description: "以树状结构显示进程关系" },
      { name: "windows.psscan.PsScan", description: "扫描隐藏/终止的进程" },
      { name: "windows.filescan.Filescan", description: "扫描 FILE 对象" },
      { name: "windows.netscan.Netscan", description: "扫描网络连接和套接字" },
      { name: "windows.handles.Handles", description: "列出进程句柄" },
      { name: "windows.dlllist.DllList", description: "列出进程加载的 DLL" },
      { name: "windows.cmdscan.CmdScan", description: "恢复命令行历史" },
      { name: "windows.consoles.Consoles", description: "恢复控制台输入缓冲区" },
      { name: "windows.svcscan.SvcScan", description: "扫描 Windows 服务" },
      { name: "windows.malfind.Malfind", description: "检测进程注入和恶意代码" },
      { name: "windows.registry.hivelist.HiveList", description: "列出注册表蜂巢" },
      { name: "windows.registry.userassist.Userassist", description: "解析 UserAssist 注册表键" },
      { name: "windows.cmdline.CmdLine", description: "显示进程命令行参数" },
      { name: "windows.envars.Envars", description: "显示进程环境变量" },
      { name: "windows.vadinfo.VadInfo", description: "显示 VAD 树信息" },
    ],
  },
  {
    name: "macOS",
    icon: "🍎",
    plugins: [
      { name: "mac.pslist.PsList", description: "列出所有进程" },
      { name: "mac.pstree.PsTree", description: "以树状结构显示进程关系" },
      { name: "mac.psscan.PsScan", description: "扫描隐藏/终止的进程" },
      { name: "mac.check_syscall.Check_syscall", description: "检查系统调用表挂钩" },
      { name: "mac.check_sysctl.Check_sysctl", description: "检查 sysctl 节点篡改" },
      { name: "mac.check_trap_table.Check_trap_table", description: "检查 trap 表挂钩" },
      { name: "mac.lsmod.Lsmod", description: "列出已加载内核扩展" },
      { name: "mac.malfind.Malfind", description: "检测进程注入和恶意代码" },
      { name: "mac.proc_maps.ProcMaps", description: "显示进程内存映射" },
      { name: "mac.bash.Bash", description: "恢复 bash 命令历史" },
    ],
  },
  {
    name: "通用",
    icon: "🔧",
    plugins: [
      { name: "banners.Banners", description: "提取内核 banner 信息" },
      { name: "isfinfo.IsfInfo", description: "显示 ISF 符号文件信息" },
      { name: "configwriter.ConfigWriter", description: "生成 Volatility 配置文件" },
      { name: "frameworkinfo.FrameworkInfo", description: "显示框架版本和配置" },
      { name: "timeliner.Timeliner", description: "生成系统活动时间线" },
    ],
  },
]

export function getAllPlugins(): PluginEntry[] {
  return pluginCategories.flatMap((cat) => cat.plugins)
}

export function findPlugin(name: string): PluginEntry | undefined {
  return getAllPlugins().find((p) => p.name === name)
}
