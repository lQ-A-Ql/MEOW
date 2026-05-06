package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DownloadMeta struct {
	URL          string `json:"url"`
	Filename     string `json:"filename"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	DownloadedAt string `json:"downloaded_at"`
	CacheHit     bool   `json:"cache_hit,omitempty"`
}

func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".meow", "cache")
	}
	return filepath.Join(".", ".meow", "cache")
}

func EnsureLayout(root string) error {
	for _, dir := range []string{
		DownloadsDir(root),
		ExtractedDir(root),
		JSONDir(root),
		SymbolsDir(root),
		MetadataDir(root),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func DownloadsDir(root string) string {
	return filepath.Join(root, "downloads")
}

func ExtractedDir(root string) string {
	return filepath.Join(root, "extracted")
}

func JSONDir(root string) string {
	return filepath.Join(root, "json")
}

func SymbolsDir(root string) string {
	return filepath.Join(root, "symbols")
}

func MetadataDir(root string) string {
	return filepath.Join(root, "metadata")
}

func CacheKey(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}

func DownloadFilePath(root, rawURL string) string {
	return filepath.Join(DownloadsDir(root), CacheKey(rawURL)+"_"+FilenameFromURL(rawURL))
}

func MetadataPath(root, rawURL string) string {
	return filepath.Join(MetadataDir(root), CacheKey(rawURL)+".json")
}

func FilenameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		if base := path.Base(parsed.Path); base != "." && base != "/" && base != "" {
			return sanitizeFilename(base)
		}
	}
	if base := path.Base(rawURL); base != "." && base != "/" && base != "" {
		return sanitizeFilename(base)
	}
	return "download.ddeb"
}

func WriteDownloadMeta(root string, meta DownloadMeta) error {
	if err := os.MkdirAll(MetadataDir(root), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(MetadataPath(root, meta.URL), append(data, '\n'), 0o644)
}

func ReadDownloadMeta(root, rawURL string) (*DownloadMeta, error) {
	data, err := os.ReadFile(MetadataPath(root, rawURL))
	if err != nil {
		return nil, err
	}
	var meta DownloadMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func ListDownloadMeta(root string) ([]DownloadMeta, error) {
	entries, err := os.ReadDir(MetadataDir(root))
	if err != nil {
		if os.IsNotExist(err) {
			return []DownloadMeta{}, nil
		}
		return nil, err
	}

	var metas []DownloadMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(MetadataDir(root), entry.Name()))
		if err != nil {
			return nil, err
		}
		var meta DownloadMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, err
		}
		metas = append(metas, meta)
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].DownloadedAt < metas[j].DownloadedAt
	})
	return metas, nil
}

func Clear(root string) error {
	if root == "" || root == "." || filepath.Clean(root) == string(filepath.Separator) {
		return fmt.Errorf("refuse to clear unsafe cache dir: %q", root)
	}
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	return EnsureLayout(root)
}

func NewDownloadMeta(rawURL, filePath, sha string, size int64, cacheHit bool) DownloadMeta {
	return DownloadMeta{
		URL:          rawURL,
		Filename:     filepath.Base(filePath),
		Path:         filePath,
		Size:         size,
		SHA256:       sha,
		DownloadedAt: time.Now().Format(time.RFC3339),
		CacheHit:     cacheHit,
	}
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}
