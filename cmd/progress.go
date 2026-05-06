package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"meow/internal/downloader"
	"meow/internal/resolver"
)

const progressWidth = 34

var pixelCatFrames = []string{
	"=^..^=__/",
	"=^..^=__\\",
	"=^..^=__/",
	"=^..^=__~",
}

func newDownloadProgress(jsonMode bool) downloader.ProgressFunc {
	if jsonMode {
		return nil
	}
	var writer terminalProgressWriter
	return func(progress downloader.Progress) {
		if progress.Done {
			writer.Clear()
			return
		}
		writer.Write(formatDownloadProgress(progress))
	}
}

func newProbeProgress(jsonMode bool) resolver.ProbeFunc {
	if jsonMode {
		return nil
	}
	var writer terminalProgressWriter
	return func(event resolver.ProbeEvent) {
		if event.Total == 0 {
			writer.Clear()
			return
		}
		writer.Write(formatProbeProgress(event))
	}
}

func formatProbeProgress(event resolver.ProbeEvent) string {
	percent := float64(event.Index) / float64(event.Total)
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	return fmt.Sprintf("[*] 探测包        [%s] %d/%d %s", formatDeterminateProgress(percent, event.Index), event.Index, event.Total, filepathBase(event.URL))
}

func formatDownloadProgress(progress downloader.Progress) string {
	downloaded := formatBytes(progress.Downloaded)
	if progress.Total <= 0 {
		return fmt.Sprintf("[*] 下载中         [%s] %s", formatIndeterminateProgress(0), downloaded)
	}
	percent := float64(progress.Downloaded) / float64(progress.Total)
	if percent > 1 {
		percent = 1
	}
	if percent < 0 {
		percent = 0
	}
	return fmt.Sprintf("[*] 下载中         [%s] %5.1f%% %s / %s", formatDeterminateProgress(percent, 0), percent*100, downloaded, formatBytes(progress.Total))
}

type stageProgress struct {
	enabled bool
	events  chan string
	extract chan extractProgressEvent
	done    chan struct{}
	once    sync.Once
	wg      sync.WaitGroup
}

type extractProgressEvent struct {
	current int
	total   int
	file    string
}

func newStageProgress(enabled bool) *stageProgress {
	if !enabled {
		return nil
	}
	p := &stageProgress{
		enabled: true,
		events:  make(chan string, 4),
		extract: make(chan extractProgressEvent, 8),
		done:    make(chan struct{}),
	}
	p.wg.Add(1)
	go p.loop()
	return p
}

func (p *stageProgress) Update(stage string) {
	if p == nil || !p.enabled {
		return
	}
	select {
	case p.events <- stage:
	default:
		select {
		case <-p.events:
		default:
		}
		p.events <- stage
	}
}

func (p *stageProgress) UpdateExtract(current, total int, file string) {
	if p == nil || !p.enabled {
		return
	}
	event := extractProgressEvent{current: current, total: total, file: file}
	select {
	case p.extract <- event:
	default:
		select {
		case <-p.extract:
		default:
		}
		p.extract <- event
	}
}

func (p *stageProgress) Close() {
	if p == nil || !p.enabled {
		return
	}
	p.once.Do(func() {
		close(p.done)
		p.wg.Wait()
	})
}

func (p *stageProgress) loop() {
	defer p.wg.Done()

	var (
		writer  terminalProgressWriter
		stage   string
		extract extractProgressEvent
		tick    int
	)
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case next := <-p.events:
			if next == "done" {
				stage = ""
				extract = extractProgressEvent{}
				writer.Clear()
				continue
			}
			stage = next
			if stage != "extract" {
				extract = extractProgressEvent{}
			}
			tick = 0
			if stage != "" {
				writer.Write(formatBuildProgress(stage, tick, extract))
			}
		case next := <-p.extract:
			extract = next
			if stage == "extract" {
				writer.Write(formatBuildProgress(stage, tick, extract))
			}
		case <-ticker.C:
			if stage != "" {
				tick++
				writer.Write(formatBuildProgress(stage, tick, extract))
			}
		case <-p.done:
			writer.Clear()
			return
		}
	}
}

type terminalProgressWriter struct {
	lastLineLen int
	lastLines   int
}

func (w *terminalProgressWriter) Write(line string) {
	lines := strings.Split(line, "\n")
	if w.lastLines > 1 {
		fmt.Fprint(os.Stderr, "\033["+fmt.Sprint(w.lastLines-1)+"A")
	}
	for i, current := range lines {
		lineLen := len([]rune(current))
		if i == len(lines)-1 {
			w.lastLineLen = lineLen
		}
		fmt.Fprint(os.Stderr, "\r"+current)
		if i < len(lines)-1 {
			fmt.Fprint(os.Stderr, "\033[K\n")
		}
	}
	if w.lastLines > len(lines) {
		for i := len(lines); i < w.lastLines; i++ {
			fmt.Fprint(os.Stderr, "\033[K\n")
		}
		fmt.Fprint(os.Stderr, "\033["+fmt.Sprint(w.lastLines-len(lines))+"A")
	}
	fmt.Fprint(os.Stderr, "\033[K")
	w.lastLines = len(lines)
}

func (w *terminalProgressWriter) Clear() {
	if w.lastLines <= 0 {
		return
	}
	if w.lastLines > 1 {
		fmt.Fprint(os.Stderr, "\033["+fmt.Sprint(w.lastLines-1)+"A")
	}
	for i := 0; i < w.lastLines; i++ {
		fmt.Fprint(os.Stderr, "\r\033[K")
		if i < w.lastLines-1 {
			fmt.Fprint(os.Stderr, "\n")
		}
	}
	if w.lastLines > 1 {
		fmt.Fprint(os.Stderr, "\033["+fmt.Sprint(w.lastLines-1)+"A")
	}
	w.lastLineLen = 0
	w.lastLines = 0
}

func formatStageProgress(stage string, tick int) string {
	return formatBuildProgress(stage, tick, extractProgressEvent{})
}

func formatBuildProgress(stage string, tick int, extract extractProgressEvent) string {
	label := stageLabel(stage)
	percent := stagePercent(stage, tick)
	top := fmt.Sprintf("[*] 构建符号       [%s] %5.1f%% %s", formatDeterminateProgress(percent, tick), percent*100, label)
	if stage != "extract" || extract.total <= 0 {
		return top
	}
	bottom := fmt.Sprintf("    解包文件       [%s] %d/%d %s", formatDeterminateProgress(extractPercent(extract), tick), extract.current, extract.total, compactPath(extract.file, 48))
	return top + "\n" + bottom
}

func formatDeterminateProgress(percent float64, tick int) string {
	cat := pixelCatFrames[tick%len(pixelCatFrames)]
	catWidth := len([]rune(cat))
	maxCatPos := progressWidth - catWidth
	if maxCatPos < 0 {
		maxCatPos = 0
	}
	catPos := int(percent * float64(maxCatPos))
	if catPos < 0 {
		catPos = 0
	}
	if catPos > maxCatPos {
		catPos = maxCatPos
	}
	return strings.Repeat("=", catPos) + cat + strings.Repeat(" ", progressWidth-catPos-catWidth)
}

func formatIndeterminateProgress(tick int) string {
	cat := pixelCatFrames[tick%len(pixelCatFrames)]
	catWidth := len([]rune(cat))
	if catWidth >= progressWidth {
		return cat[:progressWidth]
	}
	maxCatPos := progressWidth - catWidth
	catPos := tick % (maxCatPos + 1)
	return strings.Repeat(".", catPos) + cat + strings.Repeat(" ", progressWidth-catPos-catWidth)
}

func stagePercent(stage string, tick int) float64 {
	start, end := stageRange(stage)
	if end <= start {
		return start
	}
	step := float64(tick) / 80
	if step > 1 {
		step = 1
	}
	return start + (end-start)*step
}

func stageRange(stage string) (float64, float64) {
	switch stage {
	case "extract":
		return 0.05, 0.18
	case "find_vmlinux":
		return 0.18, 0.22
	case "dwarf2json":
		return 0.22, 0.82
	case "compress":
		return 0.82, 0.97
	case "move":
		return 0.97, 0.99
	default:
		return 0, 0
	}
}

func stageLabel(stage string) string {
	switch stage {
	case "extract":
		return "解包调试包"
	case "find_vmlinux":
		return "查找 vmlinux"
	case "dwarf2json":
		return "运行 dwarf2json"
	case "compress":
		return "压缩 ISF"
	case "move":
		return "收尾"
	default:
		return ""
	}
}

func extractPercent(event extractProgressEvent) float64 {
	if event.total <= 0 {
		return 0
	}
	percent := float64(event.current) / float64(event.total)
	if percent < 0 {
		return 0
	}
	if percent > 1 {
		return 1
	}
	return percent
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}

func compactPath(value string, maxRunes int) string {
	runes := []rune(value)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return "..." + string(runes[len(runes)-maxRunes+3:])
}

func filepathBase(raw string) string {
	raw = strings.TrimRight(raw, "/")
	idx := strings.LastIndex(raw, "/")
	if idx < 0 {
		return raw
	}
	return raw[idx+1:]
}
