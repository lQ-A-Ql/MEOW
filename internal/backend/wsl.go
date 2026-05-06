package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"meow/internal/runner"
)

type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	Warning bool   `json:"warning,omitempty"`
}

type WSL struct {
	Distro  string
	Verbose bool
	Stage   func(string)
	Extract func(current, total int, file string)
}

type BuildRequest struct {
	DDEBPath       string
	PackageFormat  string
	VmlinuxPath    string
	Kernel         string
	PackageVersion string
	Arch           string
	OutDir         string
	WorkDir        string
	SymbolFileName string
	Force          bool
}

type BuildOutput struct {
	SymbolPath  string
	VmlinuxPath string
	Output      string
}

func WindowsPathToWSL(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(input, "/") {
		return input, nil
	}
	abs, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	volume := filepath.VolumeName(abs)
	if len(volume) < 2 || volume[1] != ':' {
		return "", fmt.Errorf("unsupported Windows path: %s", input)
	}
	drive := strings.ToLower(volume[:1])
	rest := strings.TrimPrefix(abs, volume)
	rest = strings.TrimLeft(rest, `\/`)
	rest = filepath.ToSlash(rest)
	if rest == "" {
		return "/mnt/" + drive, nil
	}
	return "/mnt/" + drive + "/" + rest, nil
}

func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func (w WSL) BuildFromDDEB(ctx context.Context, req BuildRequest) (*BuildOutput, error) {
	if req.DDEBPath == "" {
		return nil, fmt.Errorf("missing debug package path")
	}
	debugPackage, err := WindowsPathToWSL(req.DDEBPath)
	if err != nil {
		return nil, err
	}
	format := req.PackageFormat
	if format == "" {
		format = packageFormatFromPath(req.DDEBPath)
	}
	return w.runBuild(ctx, req, map[string]string{
		"DEBUG_PACKAGE":  debugPackage,
		"PACKAGE_FORMAT": format,
	}, debugPackageBuildScript())
}

func (w WSL) BuildFromVMLINUX(ctx context.Context, req BuildRequest) (*BuildOutput, error) {
	if req.VmlinuxPath == "" {
		return nil, fmt.Errorf("missing vmlinux path")
	}
	vmlinux, err := WindowsPathToWSL(req.VmlinuxPath)
	if err != nil {
		return nil, err
	}
	return w.runBuild(ctx, req, map[string]string{"VMLINUX": vmlinux}, vmlinuxBuildScript())
}

func (w WSL) runBuild(ctx context.Context, req BuildRequest, extra map[string]string, body string) (*BuildOutput, error) {
	outDir, err := WindowsPathToWSL(req.OutDir)
	if err != nil {
		return nil, err
	}
	workDir, err := WindowsPathToWSL(req.WorkDir)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("set -euo pipefail\n")
	b.WriteString("KERNEL=" + ShellQuote(req.Kernel) + "\n")
	b.WriteString("PKGVER=" + ShellQuote(req.PackageVersion) + "\n")
	b.WriteString("ARCH=" + ShellQuote(req.Arch) + "\n")
	b.WriteString("OUT_DIR=" + ShellQuote(outDir) + "\n")
	b.WriteString("WORK_DIR=" + ShellQuote(workDir) + "\n")
	b.WriteString("SYMBOL_NAME=" + ShellQuote(req.SymbolFileName) + "\n")
	for key, value := range extra {
		b.WriteString(key + "=" + ShellQuote(value) + "\n")
	}
	b.WriteString(body)

	var result *runner.Result
	if w.Stage != nil {
		result, err = w.BashStream(ctx, b.String(), func(line string) {
			const stageMarker = "VOLSYM_STAGE="
			const extractTotalMarker = "VOLSYM_EXTRACT_TOTAL="
			const extractFileMarker = "VOLSYM_EXTRACT_FILE="
			switch {
			case strings.HasPrefix(line, stageMarker):
				w.Stage(strings.TrimSpace(strings.TrimPrefix(line, stageMarker)))
			case strings.HasPrefix(line, extractTotalMarker):
				if w.Extract != nil {
					total := parseIntMarker(line, extractTotalMarker)
					w.Extract(0, total, "")
				}
			case strings.HasPrefix(line, extractFileMarker):
				if w.Extract != nil {
					current, total, file := parseExtractFileMarker(strings.TrimPrefix(line, extractFileMarker))
					w.Extract(current, total, file)
				}
			}
		})
	} else {
		result, err = w.Bash(ctx, b.String())
	}
	if err != nil {
		return nil, err
	}
	symbolPath := filepath.Join(req.OutDir, req.SymbolFileName)
	return &BuildOutput{
		SymbolPath:  symbolPath,
		VmlinuxPath: parseMarker(result.Output, "VOLSYM_VMLINUX="),
		Output:      result.Output,
	}, nil
}

func (w WSL) Bash(ctx context.Context, script string) (*runner.Result, error) {
	binary := WSLBinary()
	args := w.bashArgs(script)
	displayArgs := w.bashArgs("<script>")
	if w.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] wsl command: %s %s\n", binary, strings.Join(displayArgs, " "))
	}
	return runner.CombinedOutputDisplay(ctx, binary+" "+strings.Join(displayArgs, " "), binary, args...)
}

func (w WSL) BashStream(ctx context.Context, script string, onLine func(string)) (*runner.Result, error) {
	binary := WSLBinary()
	args := w.bashArgs(script)
	displayArgs := w.bashArgs("<script>")
	if w.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] wsl command: %s %s\n", binary, strings.Join(displayArgs, " "))
	}
	return runner.StreamOutputDisplay(ctx, binary+" "+strings.Join(displayArgs, " "), binary, onLine, args...)
}

func (w WSL) bashArgs(script string) []string {
	args := []string{}
	if w.Distro != "" {
		args = append(args, "-d", w.Distro)
	}
	args = append(args, "--exec", "bash", "--noprofile", "--norc", "-c", script)
	return args
}

func WSLBinary() string {
	if runtime.GOOS == "windows" {
		return "wsl.exe"
	}
	return "wsl"
}

func Doctor(ctx context.Context, distro string) []Check {
	binary := WSLBinary()
	checks := []Check{
		{Name: "OS", OK: true, Detail: runtime.GOOS + "/" + runtime.GOARCH, Warning: runtime.GOOS != "windows"},
	}

	if _, err := exec.LookPath(binary); err != nil {
		checks = append(checks, Check{Name: "WSL", OK: false, Detail: err.Error()})
		return checks
	}
	checks = append(checks, Check{Name: "WSL", OK: true, Detail: binary})

	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	listArgs := []string{"-l", "-q"}
	if result, err := runner.CombinedOutput(listCtx, binary, listArgs...); err != nil {
		checks = append(checks, Check{Name: "WSL distro", OK: false, Detail: err.Error()})
	} else {
		detail := cleanWSLOutput(result.Output)
		checks = append(checks, Check{Name: "WSL distro", OK: strings.TrimSpace(detail) != "", Detail: detail})
	}

	w := WSL{Distro: distro}
	for _, dep := range []string{"bash", "dpkg-deb", "tar", "xz", "curl", "dwarf2json", "rpm2cpio", "cpio", "gzip", "zstd"} {
		depCtx, depCancel := context.WithTimeout(ctx, 10*time.Second)
		result, err := w.Bash(depCtx, "command -v "+ShellQuote(dep))
		depCancel()
		if err != nil {
			checks = append(checks, Check{Name: dep, OK: false, Detail: cleanWSLOutput(resultOutput(result, err))})
			continue
		}
		checks = append(checks, Check{Name: dep, OK: true, Detail: cleanWSLOutput(result.Output)})
	}

	return checks
}

func cleanWSLOutput(output string) string {
	output = strings.ReplaceAll(output, "\x00", "")
	lines := strings.Split(output, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(stripControl(line))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "wsl:") || strings.Contains(lower, "wsl (") || strings.Contains(lower, "loaded -") || strings.Contains(line, "已加载") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func ddebBuildScript() string {
	return debugPackageBuildScript()
}

func debugPackageBuildScript() string {
	return `
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR/extract" "$OUT_DIR"
command -v xz >/dev/null || { echo "[ERROR] missing xz" >&2; exit 21; }
command -v dwarf2json >/dev/null || { echo "[ERROR] missing dwarf2json" >&2; exit 22; }
echo "VOLSYM_STAGE=extract"
case "$PACKAGE_FORMAT" in
  rpm)
    command -v rpm2cpio >/dev/null || { echo "[ERROR] missing rpm2cpio" >&2; exit 26; }
    command -v cpio >/dev/null || { echo "[ERROR] missing cpio" >&2; exit 27; }
    echo "VOLSYM_EXTRACT_TOTAL=1"
    ( cd "$WORK_DIR/extract" && rpm2cpio "$DEBUG_PACKAGE" | cpio -idmv ) 2>&1 | while IFS= read -r extracted; do
      if [ -n "$extracted" ]; then
        echo "VOLSYM_EXTRACT_FILE=1/1:$extracted"
      fi
    done
    ;;
  deb|ddeb|unknown|"")
    command -v dpkg-deb >/dev/null || { echo "[ERROR] missing dpkg-deb" >&2; exit 20; }
    command -v tar >/dev/null || { echo "[ERROR] missing tar" >&2; exit 25; }
    EXTRACT_TOTAL="$(dpkg-deb -c "$DEBUG_PACKAGE" | awk '$1 !~ /^d/ { count++ } END { print count + 0 }')"
    echo "VOLSYM_EXTRACT_TOTAL=$EXTRACT_TOTAL"
    EXTRACT_CURRENT=0
    dpkg-deb --fsys-tarfile "$DEBUG_PACKAGE" | tar -xvf - -C "$WORK_DIR/extract" | while IFS= read -r extracted; do
      if [ -n "$extracted" ] && [ "${extracted%/}" = "$extracted" ]; then
        EXTRACT_CURRENT=$((EXTRACT_CURRENT + 1))
        echo "VOLSYM_EXTRACT_FILE=$EXTRACT_CURRENT/$EXTRACT_TOTAL:$extracted"
      fi
    done
    ;;
  *)
    echo "[ERROR] unsupported debug package format: $PACKAGE_FORMAT" >&2
    exit 28
    ;;
esac
echo "VOLSYM_STAGE=find_vmlinux"
VMLINUX=""
for candidate in \
  "$WORK_DIR/extract/usr/lib/debug/boot/vmlinux-$KERNEL" \
  "$WORK_DIR/extract/usr/lib/debug/lib/modules/$KERNEL/vmlinux" \
  "$WORK_DIR/extract/usr/lib/debug/lib64/modules/$KERNEL/vmlinux"; do
  if [ -f "$candidate" ]; then
    VMLINUX="$candidate"
    break
  fi
done
if [ -z "$VMLINUX" ]; then
  VMLINUX="$(find "$WORK_DIR/extract" -type f \( -name "vmlinux" -o -name "vmlinux-*" -o -name "vmlinux*.gz" -o -name "vmlinux*.xz" -o -name "vmlinux*.zst" \) | head -n 1 || true)"
fi
if [ -z "$VMLINUX" ]; then
  echo "[ERROR] debug package extracted but vmlinux not found" >&2
  find "$WORK_DIR/extract" -type f | head -n 50 >&2 || true
  exit 23
fi
case "$VMLINUX" in
  *.gz)
    command -v gzip >/dev/null || { echo "[ERROR] missing gzip" >&2; exit 29; }
    gzip -dc "$VMLINUX" > "$WORK_DIR/vmlinux"
    VMLINUX="$WORK_DIR/vmlinux"
    ;;
  *.xz)
    xz -dc "$VMLINUX" > "$WORK_DIR/vmlinux"
    VMLINUX="$WORK_DIR/vmlinux"
    ;;
  *.zst)
    command -v zstd >/dev/null || { echo "[ERROR] missing zstd" >&2; exit 30; }
    zstd -dc "$VMLINUX" > "$WORK_DIR/vmlinux"
    VMLINUX="$WORK_DIR/vmlinux"
    ;;
esac
echo "VOLSYM_VMLINUX=$VMLINUX"
echo "VOLSYM_STAGE=dwarf2json"
dwarf2json linux --elf "$VMLINUX" > "$WORK_DIR/symbol.json"
echo "VOLSYM_STAGE=compress"
xz -T0 -f -z "$WORK_DIR/symbol.json"
echo "VOLSYM_STAGE=move"
mv -f "$WORK_DIR/symbol.json.xz" "$OUT_DIR/$SYMBOL_NAME"
echo "VOLSYM_SYMBOL=$OUT_DIR/$SYMBOL_NAME"
echo "VOLSYM_STAGE=done"
`
}

func packageFormatFromPath(filePath string) string {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".rpm"):
		return "rpm"
	case strings.HasSuffix(lower, ".deb"):
		return "deb"
	case strings.HasSuffix(lower, ".ddeb"):
		return "ddeb"
	default:
		return "unknown"
	}
}

func vmlinuxBuildScript() string {
	return `
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR" "$OUT_DIR"
command -v xz >/dev/null || { echo "[ERROR] missing xz" >&2; exit 21; }
command -v dwarf2json >/dev/null || { echo "[ERROR] missing dwarf2json" >&2; exit 22; }
test -f "$VMLINUX" || { echo "[ERROR] vmlinux not found: $VMLINUX" >&2; exit 24; }
echo "VOLSYM_VMLINUX=$VMLINUX"
echo "VOLSYM_STAGE=dwarf2json"
dwarf2json linux --elf "$VMLINUX" > "$WORK_DIR/symbol.json"
echo "VOLSYM_STAGE=compress"
xz -T0 -f -z "$WORK_DIR/symbol.json"
echo "VOLSYM_STAGE=move"
mv -f "$WORK_DIR/symbol.json.xz" "$OUT_DIR/$SYMBOL_NAME"
echo "VOLSYM_SYMBOL=$OUT_DIR/$SYMBOL_NAME"
echo "VOLSYM_STAGE=done"
`
}

func parseMarker(output, marker string) string {
	re := regexp.MustCompile(regexp.QuoteMeta(marker) + `([^\r\n]+)`)
	match := re.FindStringSubmatch(output)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func parseIntMarker(line, marker string) int {
	value := strings.TrimSpace(strings.TrimPrefix(line, marker))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func parseExtractFileMarker(value string) (int, int, string) {
	head, file, _ := strings.Cut(value, ":")
	currentText, totalText, _ := strings.Cut(head, "/")
	current, _ := strconv.Atoi(strings.TrimSpace(currentText))
	total, _ := strconv.Atoi(strings.TrimSpace(totalText))
	return current, total, strings.TrimSpace(file)
}

func resultOutput(result *runner.Result, err error) string {
	if result != nil && result.Output != "" {
		return result.Output
	}
	if err != nil {
		return err.Error()
	}
	return ""
}
