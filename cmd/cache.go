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
	cacheDir   string
	cacheJSON  bool
	cacheForce bool
)

func init() {
	fs := Register("cache", "view or clear download cache", runCache)
	fs.StringVar(&cacheDir, "cache-dir", cachepkg.DefaultDir(), "cache directory")
	fs.BoolVar(&cacheForce, "force", false, "force clear custom cache dir")
	fs.BoolVar(&cacheJSON, "json", false, "output JSON")
}

func runCache(args []string) {
	args = absorbTrailingJSONFlag(args, &cacheJSON)
	jsonMode := cacheJSON || JSONFlag
	applyCacheConfigDefaults(jsonMode)
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
			log.Info("cache is empty: %s", cacheDir)
			return
		}
		for _, meta := range metas {
			fmt.Printf("%s  %d bytes  %s\n", meta.Filename, meta.Size, meta.URL)
		}
	case "clear":
		if err := cachepkg.Clear(cacheDir, cacheForce); err != nil {
			cacheFail(jsonMode, err)
		}
		if jsonMode {
			data, _ := json.MarshalIndent(map[string]string{"status": "cleared", "cache_dir": cacheDir}, "", "  ")
			fmt.Println(string(data))
			return
		}
		log.Success("cache cleared: %s", cacheDir)
	default:
		cacheFail(jsonMode, fmt.Errorf("unknown cache subcommand: %s", sub))
	}
}

func applyCacheConfigDefaults(jsonMode bool) {
	cfg, err := readOrDefaultConfig()
	if err != nil {
		if !jsonMode {
			log.Warn("failed to read config defaults: %v", err)
		}
		return
	}
	if !flagWasSet(Commands["cache"].Flags, "cache-dir") {
		cacheDir = cfg.CacheDir
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
