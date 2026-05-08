package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	backendpkg "meow/internal/backend"
	"meow/internal/log"
)

var (
	doctorJSON bool
)

func init() {
	fs := Register("doctor", "检查 Linux 原生依赖", runDoctor)
	fs.BoolVar(&log.Verbose, "verbose", false, "输出详细日志")
	fs.BoolVar(&doctorJSON, "json", false, "以 JSON 格式输出")
}

func runDoctor(args []string) {
	jsonMode := doctorJSON || JSONFlag
	if runtime.GOOS != "linux" {
		err := fmt.Errorf("当前版本仅支持 Linux 原生运行")
		if jsonMode {
			data, _ := json.MarshalIndent(map[string]string{"error": err.Error()}, "", "  ")
			fmt.Println(string(data))
		} else {
			log.Fatal("%v", err)
		}
		os.Exit(1)
	}

	checks := backendpkg.Doctor(context.Background())
	if jsonMode {
		data, _ := json.MarshalIndent(checks, "", "  ")
		fmt.Println(string(data))
		return
	}

	buildOK := true
	verifyOK := false
	for _, check := range checks {
		if check.OK {
			if check.Warning {
				log.Warn("%s: %s", check.Name, check.Detail)
			} else {
				log.Success("%s: %s", check.Name, check.Detail)
			}
		} else {
			log.Warn("%s: %s", check.Name, check.Detail)
			buildOK = false
		}
		if check.Name == "dwarf2json" && check.OK {
			verifyOK = true
		}
	}
	if buildOK {
		log.Success("Result: build available")
	} else {
		log.Warn("Result: build unavailable")
	}
	if verifyOK {
		log.Success("Result: verify tooling partially available")
	}
}
