package volatility

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"meow/internal/runner"
)

var linuxBannerRe = regexp.MustCompile(`Linux version[^\r\n]+`)

func ExtractBanner(ctx context.Context, volPath, memPath string) (string, string, error) {
	if volPath == "" {
		volPath = "vol"
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	result, err := runner.CombinedOutput(ctx, volPath, "-f", memPath, "banners.Banners")
	if err != nil {
		return "", resultText(result), fmt.Errorf("Volatility banners.Banners failed: %w", err)
	}
	banner := linuxBannerRe.FindString(result.Output)
	if banner == "" {
		return "", result.Output, fmt.Errorf("no Linux banner found in Volatility output")
	}
	return strings.TrimSpace(banner), result.Output, nil
}

func Verify(ctx context.Context, volPath, memPath, symbolsPath string) (string, error) {
	if volPath == "" {
		volPath = "vol"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var out strings.Builder
	banners, err := runner.CombinedOutput(ctx, volPath, "-f", memPath, "-s", symbolsPath, "linux.banners.Banners")
	if banners != nil {
		out.WriteString(banners.Output)
	}
	if err != nil {
		return out.String(), fmt.Errorf("linux.banners.Banners failed: %w", err)
	}

	pslist, err := runner.CombinedOutput(ctx, volPath, "-f", memPath, "-s", symbolsPath, "linux.pslist.PsList")
	if pslist != nil {
		out.WriteString(pslist.Output)
	}
	if err != nil {
		return out.String(), fmt.Errorf("linux.pslist.PsList failed: %w", err)
	}
	return out.String(), nil
}

func resultText(result *runner.Result) string {
	if result == nil {
		return ""
	}
	return result.Output
}
