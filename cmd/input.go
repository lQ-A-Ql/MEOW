package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func readBannerFromTerminal(jsonMode bool) (string, error) {
	if !jsonMode {
		fmt.Fprintln(os.Stderr, "请粘贴 Linux kernel banner 后按 Enter:")
	}
	return readBanner(os.Stdin)
}

func readBanner(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("未读取到 banner")
	}
	banner := strings.TrimSpace(scanner.Text())
	if banner == "" {
		return "", fmt.Errorf("banner 为空")
	}
	return banner, nil
}
