package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"meow/internal/log"
	"meow/internal/volatility"
)

var (
	verifyMem     string
	verifySymbols string
	verifyVolPath string
	verifyJSON    bool
)

func init() {
	fs := Register("verify", "用 Volatility 3 验证符号目录", runVerify)
	fs.StringVar(&verifyMem, "mem", "", "内存镜像路径")
	fs.StringVar(&verifySymbols, "symbols", filepathDefaultSymbols(), "symbols 目录")
	fs.StringVar(&verifyVolPath, "vol", "vol", "Volatility 3 命令路径")
	fs.BoolVar(&log.Verbose, "verbose", false, "输出详细日志")
	fs.BoolVar(&verifyJSON, "json", false, "以 JSON 格式输出")
}

func runVerify(args []string) {
	jsonMode := verifyJSON || JSONFlag
	if verifyMem == "" {
		verifyFail(jsonMode, "", fmt.Errorf("需要 --mem 参数"))
	}

	output, err := volatility.Verify(context.Background(), verifyVolPath, verifyMem, verifySymbols)
	if err != nil {
		verifyFail(jsonMode, output, err)
	}

	if jsonMode {
		data, _ := json.MarshalIndent(map[string]any{
			"success": true,
			"output":  output,
		}, "", "  ")
		fmt.Println(string(data))
		return
	}
	log.Success("Volatility 3 loaded symbol table successfully.")
	log.Success("linux.pslist.PsList executed successfully.")
}

func verifyFail(jsonMode bool, output string, err error) {
	if jsonMode {
		data, _ := json.MarshalIndent(map[string]any{
			"success": false,
			"error":   err.Error(),
			"output":  output,
		}, "", "  ")
		fmt.Println(string(data))
	} else {
		log.Error("符号表验证失败: %v", err)
		fmt.Fprintln(os.Stderr, "Possible causes:")
		fmt.Fprintln(os.Stderr, "  1. symbols/linux 目录层级错误。")
		fmt.Fprintln(os.Stderr, "  2. json.xz 文件损坏。")
		fmt.Fprintln(os.Stderr, "  3. banner 不匹配。")
		fmt.Fprintln(os.Stderr, "  4. Volatility 3 缓存未清理。")
		fmt.Fprintln(os.Stderr, "建议执行：meow cache clear")
	}
	os.Exit(1)
}

func filepathDefaultSymbols() string {
	return "./symbols"
}
