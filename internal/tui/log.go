package tui

import (
	"fmt"
	"strings"
	"time"
)

type LogLevel string

const (
	LogInfo    LogLevel = "info"
	LogSuccess LogLevel = "success"
	LogWarn    LogLevel = "warn"
	LogError   LogLevel = "error"
	LogStdout  LogLevel = "stdout"
	LogStderr  LogLevel = "stderr"
)

type LogEntry struct {
	Level   LogLevel
	Message string
	Time    string
}

const maxLogs = 500

type LogStore struct {
	entries []LogEntry
}

func NewLogStore() *LogStore {
	return &LogStore{}
}

func (ls *LogStore) Append(level LogLevel, message string) {
	ls.entries = append(ls.entries, LogEntry{
		Level:   level,
		Message: message,
		Time:    time.Now().Format("15:04:05"),
	})
	if len(ls.entries) > maxLogs {
		ls.entries = ls.entries[len(ls.entries)-maxLogs:]
	}
}

func (ls *LogStore) AppendChunk(level LogLevel, chunk string) {
	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			ls.Append(level, line)
		}
	}
}

func (ls *LogStore) Entries() []LogEntry {
	return ls.entries
}

func (ls *LogStore) Clear() {
	ls.entries = nil
}

func (ls *LogStore) RenderLines() []string {
	lines := make([]string, 0, len(ls.entries))
	for _, e := range ls.entries {
		style := logColor(e.Level)
		lines = append(lines, style.Render(fmt.Sprintf("[%s] %s", e.Time, e.Message)))
	}
	return lines
}

func (ls *LogStore) LastN(n int) []LogEntry {
	if n >= len(ls.entries) {
		return ls.entries
	}
	return ls.entries[len(ls.entries)-n:]
}
