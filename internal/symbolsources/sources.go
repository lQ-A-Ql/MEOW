package symbolsources

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultName     = "abyss"
	DefaultIndexURL = "https://raw.githubusercontent.com/Abyss-W4tcher/volatility3-symbols/master/banners/banners_plain.json"
	DefaultRawBase  = "https://raw.githubusercontent.com/Abyss-W4tcher/volatility3-symbols/master/"
)

type Source struct {
	Name       string `json:"name"`
	IndexURL   string `json:"index_url"`
	RawBaseURL string `json:"raw_base_url"`
}

type Match struct {
	Source     Source `json:"source"`
	Banner     string `json:"banner,omitempty"`
	SymbolPath string `json:"symbol_path"`
	URL        string `json:"url"`
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".meow", "symbol-sources.txt")
	}
	return filepath.Join(home, ".meow", "symbol-sources.txt")
}

func DefaultSources() []Source {
	return []Source{{
		Name:       DefaultName,
		IndexURL:   DefaultIndexURL,
		RawBaseURL: DefaultRawBase,
	}}
}

func DefaultFileContent() string {
	return "# name|index_url|raw_base_url\n" +
		DefaultName + "|" + DefaultIndexURL + "|" + DefaultRawBase + "\n"
}

func Load(filePath string) ([]Source, error) {
	if filePath == "" {
		filePath = DefaultPath()
	}
	file, err := os.Open(filePath)
	if os.IsNotExist(err) {
		return DefaultSources(), nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sources []Source
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			return nil, fmt.Errorf("%s:%d: expected name|index_url|raw_base_url", filePath, lineNo)
		}
		src := Source{
			Name:       strings.TrimSpace(parts[0]),
			IndexURL:   strings.TrimSpace(parts[1]),
			RawBaseURL: strings.TrimSpace(parts[2]),
		}
		if src.Name == "" || src.IndexURL == "" || src.RawBaseURL == "" {
			return nil, fmt.Errorf("%s:%d: source fields cannot be empty", filePath, lineNo)
		}
		sources = append(sources, src)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return sources, nil
}

func Find(ctx context.Context, client *http.Client, sources []Source, banner string) (*Match, []string, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	var warnings []string
	for _, src := range sources {
		match, err := findInSource(ctx, client, src, banner)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", src.Name, err))
			continue
		}
		if match != nil {
			return match, warnings, nil
		}
	}
	return nil, warnings, nil
}

func findInSource(ctx context.Context, client *http.Client, src Source, banner string) (*Match, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.IndexURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("index fetch failed: %s", resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	index, err := parseBannerIndex(raw)
	if err != nil {
		return nil, err
	}
	relative, ok := index[banner]
	if !ok {
		return nil, nil
	}
	return &Match{
		Source:     src,
		Banner:     banner,
		SymbolPath: relative,
		URL:        JoinRawURL(src.RawBaseURL, relative),
	}, nil
}

func JoinRawURL(base, relative string) string {
	if strings.HasPrefix(relative, "http://") || strings.HasPrefix(relative, "https://") {
		return relative
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(relative, "/")
}

func SymbolFileName(symbolPath string) string {
	base := path.Base(symbolPath)
	if base == "." || base == "/" || base == "" {
		return "symbol.json.xz"
	}
	return base
}

func parseBannerIndex(raw []byte) (map[string]string, error) {
	var single map[string]string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single, nil
	}

	var multiple map[string][]string
	if err := json.Unmarshal(raw, &multiple); err != nil {
		return nil, err
	}
	index := make(map[string]string, len(multiple))
	for banner, paths := range multiple {
		if len(paths) == 0 {
			continue
		}
		index[banner] = paths[0]
	}
	return index, nil
}
