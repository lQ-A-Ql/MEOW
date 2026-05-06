package cmd

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"meow/internal/log"
)

var (
	Commands    = make(map[string]*SubCommand)
	VerboseFlag bool
	JSONFlag    bool
)

type SubCommand struct {
	Name    string
	Desc    string
	Flags   *flag.FlagSet
	Handler func(args []string)
}

func Register(name, desc string, handler func(args []string)) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ExitOnError)
	Commands[name] = &SubCommand{Name: name, Desc: desc, Flags: f, Handler: handler}
	return f
}

func Execute() {
	flag.Usage = printUsage

	cmdName, cmdArgs, ok := parseGlobalFlags(os.Args[1:])
	if !ok {
		printUsage()
		os.Exit(1)
	}

	cmd, ok := Commands[cmdName]
	if !ok {
		fmt.Fprintf(os.Stderr, "[ERROR] 未知命令: %s\n\n", cmdName)
		printUsage()
		os.Exit(1)
	}

	cmd.Flags.Parse(cmdArgs)
	cmd.Handler(cmd.Flags.Args())
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "用法:\n")
	fmt.Fprintf(os.Stderr, "  meow [全局参数] <命令> [命令参数]\n\n")
	fmt.Fprintf(os.Stderr, "命令:\n")
	names := make([]string, 0, len(Commands))
	for name := range Commands {
		names = append(names, name)
	}
	sort.Strings(names)
	width := 0
	for _, name := range names {
		if len(name) > width {
			width = len(name)
		}
	}
	for _, name := range names {
		cmd := Commands[name]
		fmt.Fprintf(os.Stderr, "  %-*s  %s\n", width, name, cmd.Desc)
	}
	fmt.Fprintf(os.Stderr, "\n全局参数:\n")
	fmt.Fprintf(os.Stderr, "  --verbose  输出调试日志\n")
	fmt.Fprintf(os.Stderr, "  --json     输出纯 JSON；不显示 logo / 进度条\n")
	fmt.Fprintf(os.Stderr, "  -h, --help 显示帮助\n")
	fmt.Fprintf(os.Stderr, "\n常用:\n")
	fmt.Fprintf(os.Stderr, "  meow parse\n")
	fmt.Fprintf(os.Stderr, "  meow build --dry-run\n")
	fmt.Fprintf(os.Stderr, "  meow build --debug-package local.rpm\n")
	fmt.Fprintf(os.Stderr, "  meow build --repo-url https://mirror.example/debug/os/x86_64/\n")
}

func parseGlobalFlags(args []string) (string, []string, bool) {
	for len(args) > 0 {
		arg := args[0]
		switch arg {
		case "--json":
			JSONFlag = true
			args = args[1:]
		case "--verbose":
			VerboseFlag = true
			log.Verbose = true
			args = args[1:]
		case "-h", "--help", "-help":
			return "", nil, false
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "[ERROR] 未知全局参数: %s\n\n", arg)
				return "", nil, false
			}
			return arg, args[1:], true
		}
	}
	return "", nil, false
}
