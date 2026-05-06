package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	cachepkg "meow/internal/cache"
	"meow/internal/log"
	sourcespkg "meow/internal/symbolsources"
)

type configFile struct {
	Backend                string `json:"backend"`
	WSLDistro              string `json:"wsl_distro"`
	CacheDir               string `json:"cache_dir"`
	OutputDir              string `json:"output_dir"`
	SymbolSourcesPath      string `json:"symbol_sources_path"`
	VolatilityPath         string `json:"volatility_path"`
	AutoClearVolCache      bool   `json:"auto_clear_vol_cache"`
	DownloadTimeoutSeconds int    `json:"download_timeout_seconds"`
	MaxRetries             int    `json:"max_retries"`
}

var configJSON bool

func init() {
	fs := Register("config", "查看/初始化配置与符号源", runConfig)
	fs.BoolVar(&configJSON, "json", false, "以 JSON 格式输出")
}

func runConfig(args []string) {
	args = absorbTrailingJSONFlag(args, &configJSON)
	jsonMode := configJSON || JSONFlag
	sub := "show"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "path":
		path := defaultConfigPath()
		if jsonMode {
			printJSON(map[string]string{"config_path": path})
			return
		}
		fmt.Println(path)
	case "show":
		cfg, err := readOrDefaultConfig()
		if err != nil {
			failConfig(jsonMode, err)
		}
		if jsonMode {
			printJSON(cfg)
			return
		}
		fmt.Printf("backend: %s\n", cfg.Backend)
		fmt.Printf("wsl_distro: %s\n", cfg.WSLDistro)
		fmt.Printf("cache_dir: %s\n", cfg.CacheDir)
		fmt.Printf("output_dir: %s\n", cfg.OutputDir)
		fmt.Printf("symbol_sources_path: %s\n", cfg.SymbolSourcesPath)
		fmt.Printf("volatility_path: %s\n", cfg.VolatilityPath)
	case "init":
		path := defaultConfigPath()
		if _, err := os.Stat(path); err == nil {
			failConfig(jsonMode, fmt.Errorf("配置已存在: %s", path))
		}
		cfg := defaultConfig()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			failConfig(jsonMode, err)
		}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			failConfig(jsonMode, err)
		}
		sourcesPath := sourcespkg.DefaultPath()
		if err := os.MkdirAll(filepath.Dir(sourcesPath), 0o755); err != nil {
			failConfig(jsonMode, err)
		}
		if _, err := os.Stat(sourcesPath); os.IsNotExist(err) {
			if err := os.WriteFile(sourcesPath, []byte(sourcespkg.DefaultFileContent()), 0o644); err != nil {
				failConfig(jsonMode, err)
			}
		} else if err != nil {
			failConfig(jsonMode, err)
		}
		if jsonMode {
			printJSON(map[string]string{"status": "created", "config_path": path, "symbol_sources_path": sourcesPath})
			return
		}
		log.Success("配置已创建: %s", path)
		log.Success("符号源已创建: %s", sourcesPath)
	default:
		failConfig(jsonMode, fmt.Errorf("未知 config 子命令: %s", sub))
	}
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".meow", "config.json")
	}
	return filepath.Join(home, ".meow", "config.json")
}

func defaultConfig() configFile {
	return configFile{
		Backend:                "wsl",
		CacheDir:               cachepkg.DefaultDir(),
		OutputDir:              filepath.Join(".", "symbols", "linux"),
		SymbolSourcesPath:      sourcespkg.DefaultPath(),
		VolatilityPath:         "vol",
		AutoClearVolCache:      false,
		DownloadTimeoutSeconds: 60,
		MaxRetries:             3,
	}
}

func readOrDefaultConfig() (configFile, error) {
	path := defaultConfigPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultConfig(), nil
	}
	if err != nil {
		return configFile{}, err
	}
	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return configFile{}, err
	}
	if cfg.SymbolSourcesPath == "" {
		cfg.SymbolSourcesPath = sourcespkg.DefaultPath()
	}
	return cfg, nil
}

func failConfig(jsonMode bool, err error) {
	if jsonMode {
		printJSON(map[string]string{"error": err.Error()})
	} else {
		log.Error("%v", err)
	}
	os.Exit(1)
}

func printJSON(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}
