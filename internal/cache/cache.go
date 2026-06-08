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

const SentinelFileName = ".meow-cache"

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
	if err := os.WriteFile(SentinelPath(root), []byte("meow cache\n"), 0o644); err != nil {
		return err
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

func SentinelPath(root string) string {
	return filepath.Join(root, SentinelFileName)
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

func Clear(root string, force bool) error {
	if err := validateClearRoot(root, force); err != nil {
		return err
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

func validateClearRoot(root string, force bool) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("refuse to clear unsafe cache dir: %q", root)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	clean := filepath.Clean(abs)
	if clean == "." || clean == string(filepath.Separator) {
		return fmt.Errorf("refuse to clear unsafe cache dir: %q", root)
	}
	volume := filepath.VolumeName(clean)
	if volume != "" && clean == volume+string(filepath.Separator) {
		return fmt.Errorf("refuse to clear unsafe cache dir: %q", root)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		homeAbs, err := filepath.Abs(home)
		if err == nil && samePath(clean, filepath.Clean(homeAbs)) {
			return fmt.Errorf("refuse to clear home directory as cache dir: %s", clean)
		}
	}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		cwdAbs, err := filepath.Abs(cwd)
		if err == nil && pathContains(clean, filepath.Clean(cwdAbs)) {
			return fmt.Errorf("refuse to clear current working directory or ancestor as cache dir: %s", clean)
		}
	}
	defaultAbs, err := filepath.Abs(DefaultDir())
	isDefault := err == nil && samePath(clean, filepath.Clean(defaultAbs))
	if !isDefault && !force {
		if _, err := os.Stat(SentinelPath(clean)); err != nil {
			return fmt.Errorf("refuse to clear custom cache dir without %s sentinel: %s", SentinelFileName, clean)
		}
	}
	return nil
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func pathContains(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if samePath(parent, child) {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
