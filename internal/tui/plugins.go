package tui

type PluginEntry struct {
	Name        string
	Description string
}

type PluginCategory struct {
	Name    string
	Icon    string
	Plugins []PluginEntry
}

var PluginCategories = []PluginCategory{
	{
		Name: "Linux", Icon: "🐧",
		Plugins: []PluginEntry{
			{"linux.pslist.PsList", "列出所有进程 (EPROCESS)"},
			{"linux.pstree.PsTree", "树状结构显示进程关系"},
			{"linux.psscan.PsScan", "扫描隐藏/终止的进程"},
			{"linux.files.Files", "列出所有打开的文件描述符"},
			{"linux.proc_maps.ProcMaps", "显示进程内存映射"},
			{"linux.bash.Bash", "从 bash_history 恢复命令历史"},
			{"linux.lsmod.Lsmod", "列出已加载内核模块"},
			{"linux.lsof.Lsof", "列出所有进程打开的文件"},
			{"linux.sockstat.Sockstat", "显示网络套接字状态"},
			{"linux.check_syscall.Check_syscall", "检查系统调用表挂钩"},
			{"linux.check_modules.Check_modules", "检测未列出的内核模块"},
			{"linux.malfind.Malfind", "检测进程注入和恶意代码"},
		},
	},
	{
		Name: "通用", Icon: "🔧",
		Plugins: []PluginEntry{
			{"banners.Banners", "提取内核 banner 信息"},
			{"isfinfo.IsfInfo", "显示 ISF 符号文件信息"},
			{"timeliner.Timeliner", "生成系统活动时间线"},
		},
	},
}

func AllPlugins() []PluginEntry {
	var all []PluginEntry
	for _, cat := range PluginCategories {
		all = append(all, cat.Plugins...)
	}
	return all
}
