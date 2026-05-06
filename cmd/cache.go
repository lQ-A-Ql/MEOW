package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	cachepkg "meow/internal/cache"
	"meow/internal/log"
)

var (
	cacheDir  string
	cacheJSON bool
)

func init() {
	fs := Register("cache", "查看/清理下载缓存", runCache)
	fs.StringVar(&cacheDir, "cache-dir", cachepkg.DefaultDir(), "缓存目录")
	fs.BoolVar(&cacheJSON, "json", false, "以 JSON 格式输出")
}

func runCache(args []string) {
	args = absorbTrailingJSONFlag(args, &cacheJSON)
	jsonMode := cacheJSON || JSONFlag
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "path":
		if jsonMode {
			data, _ := json.MarshalIndent(map[string]string{"cache_dir": cacheDir}, "", "  ")
			fmt.Println(string(data))
			return
		}
		fmt.Println(cacheDir)
	case "list":
		metas, err := cachepkg.ListDownloadMeta(cacheDir)
		if err != nil {
			cacheFail(jsonMode, err)
		}
		if jsonMode {
			data, _ := json.MarshalIndent(metas, "", "  ")
			fmt.Println(string(data))
			return
		}
		if len(metas) == 0 {
			log.Info("缓存为空: %s", cacheDir)
			return
		}
		for _, meta := range metas {
			fmt.Printf("%s  %d bytes  %s\n", meta.Filename, meta.Size, meta.URL)
		}
	case "clear":
		if err := cachepkg.Clear(cacheDir); err != nil {
			cacheFail(jsonMode, err)
		}
		if jsonMode {
			data, _ := json.MarshalIndent(map[string]string{"status": "cleared", "cache_dir": cacheDir}, "", "  ")
			fmt.Println(string(data))
			return
		}
		log.Success("缓存已清理: %s", cacheDir)
	default:
		cacheFail(jsonMode, fmt.Errorf("未知 cache 子命令: %s", sub))
	}
}

func cacheFail(jsonMode bool, err error) {
	if jsonMode {
		data, _ := json.MarshalIndent(map[string]string{"error": err.Error()}, "", "  ")
		fmt.Println(string(data))
	} else {
		log.Error("%v", err)
	}
	os.Exit(1)
}

func absorbTrailingJSONFlag(args []string, target *bool) []string {
	out := args[:0]
	for _, arg := range args {
		if arg == "--json" {
			*target = true
			continue
		}
		if strings.TrimSpace(arg) == "" {
			continue
		}
		out = append(out, arg)
	}
	return out
}
