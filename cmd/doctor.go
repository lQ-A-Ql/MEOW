package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	backendpkg "meow/internal/backend"
	"meow/internal/log"
)

var (
	doctorBackend   string
	doctorWslDistro string
	doctorJSON      bool
)

func init() {
	fs := Register("doctor", "检查 Windows/WSL/dwarf2json 依赖", runDoctor)
	fs.StringVar(&doctorBackend, "backend", "wsl", "后端: wsl/native")
	fs.StringVar(&doctorWslDistro, "wsl-distro", "", "WSL 发行版名称，空则使用默认")
	fs.BoolVar(&log.Verbose, "verbose", false, "输出详细日志")
	fs.BoolVar(&doctorJSON, "json", false, "以 JSON 格式输出")
}

func runDoctor(args []string) {
	jsonMode := doctorJSON || JSONFlag
	if doctorBackend != "wsl" {
		err := fmt.Errorf("native backend 尚未支持；MVP 请使用 --backend wsl")
		if jsonMode {
			data, _ := json.MarshalIndent(map[string]string{"error": err.Error()}, "", "  ")
			fmt.Println(string(data))
		} else {
			log.Fatal("%v", err)
		}
		os.Exit(1)
	}

	checks := backendpkg.Doctor(context.Background(), doctorWslDistro)
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
			if check.Name != "curl" {
				buildOK = false
			}
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
