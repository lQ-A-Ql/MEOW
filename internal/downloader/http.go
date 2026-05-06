package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	cachepkg "meow/internal/cache"
)

type Result struct {
	cachepkg.DownloadMeta
}

type Progress struct {
	Downloaded int64
	Total      int64
	Done       bool
}

type ProgressFunc func(Progress)

func Download(ctx context.Context, client *http.Client, rawURL, dest string, force bool, progress ProgressFunc) (*Result, error) {
	if client == nil {
		client = &http.Client{}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, err
	}

	if !force {
		if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
			sha, err := fileSHA256(dest)
			if err != nil {
				return nil, err
			}
			meta := cachepkg.NewDownloadMeta(rawURL, dest, sha, info.Size(), true)
			return &Result{DownloadMeta: meta}, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}

	tmp := dest + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	hasher := sha256.New()
	progressReader := &progressReader{
		reader:   resp.Body,
		total:    resp.ContentLength,
		progress: progress,
	}
	written, copyErr := io.Copy(io.MultiWriter(out, hasher), progressReader)
	progressReader.finish()
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return nil, closeErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}

	meta := cachepkg.NewDownloadMeta(rawURL, dest, hex.EncodeToString(hasher.Sum(nil)), written, false)
	return &Result{DownloadMeta: meta}, nil
}

type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	progress   ProgressFunc
	lastEmit   time.Time
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.downloaded += int64(n)
		r.emit(false)
	}
	return n, err
}

func (r *progressReader) finish() {
	r.emit(true)
}

func (r *progressReader) emit(done bool) {
	if r.progress == nil {
		return
	}
	now := time.Now()
	if !done && !r.lastEmit.IsZero() && now.Sub(r.lastEmit) < 250*time.Millisecond {
		return
	}
	r.lastEmit = now
	r.progress(Progress{
		Downloaded: r.downloaded,
		Total:      r.total,
		Done:       done,
	})
}

func fileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
