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

	"meow/internal/runner"
)

type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
	Warning bool   `json:"warning,omitempty"`
}

type Native struct {
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

func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func (n Native) BuildFromDDEB(ctx context.Context, req BuildRequest) (*BuildOutput, error) {
	return n.BuildFromDebugPackage(ctx, req)
}

func (n Native) BuildFromDebugPackage(ctx context.Context, req BuildRequest) (*BuildOutput, error) {
	if req.DDEBPath == "" {
		return nil, fmt.Errorf("missing debug package path")
	}
	debugPackage, err := filepath.Abs(req.DDEBPath)
	if err != nil {
		return nil, err
	}
	format := req.PackageFormat
	if format == "" {
		format = packageFormatFromPath(debugPackage)
	}
	return n.runBuild(ctx, req, map[string]string{
		"DEBUG_PACKAGE":  debugPackage,
		"PACKAGE_FORMAT": format,
	}, debugPackageBuildScript())
}

func (n Native) BuildFromVMLINUX(ctx context.Context, req BuildRequest) (*BuildOutput, error) {
	if req.VmlinuxPath == "" {
		return nil, fmt.Errorf("missing vmlinux path")
	}
	vmlinux, err := filepath.Abs(req.VmlinuxPath)
	if err != nil {
		return nil, err
	}
	return n.runBuild(ctx, req, map[string]string{"VMLINUX": vmlinux}, vmlinuxBuildScript())
}

func (n Native) runBuild(ctx context.Context, req BuildRequest, extra map[string]string, body string) (*BuildOutput, error) {
	outDir, err := filepath.Abs(req.OutDir)
	if err != nil {
		return nil, err
	}
	workDir, err := filepath.Abs(req.WorkDir)
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
	if n.Stage != nil {
		result, err = n.BashStream(ctx, b.String(), func(line string) {
			const stageMarker = "VOLSYM_STAGE="
			const extractTotalMarker = "VOLSYM_EXTRACT_TOTAL="
			const extractFileMarker = "VOLSYM_EXTRACT_FILE="
			switch {
			case strings.HasPrefix(line, stageMarker):
				n.Stage(strings.TrimSpace(strings.TrimPrefix(line, stageMarker)))
			case strings.HasPrefix(line, extractTotalMarker):
				if n.Extract != nil {
					total := parseIntMarker(line, extractTotalMarker)
					n.Extract(0, total, "")
				}
			case strings.HasPrefix(line, extractFileMarker):
				if n.Extract != nil {
					current, total, file := parseExtractFileMarker(strings.TrimPrefix(line, extractFileMarker))
					n.Extract(current, total, file)
				}
			}
		})
	} else {
		result, err = n.Bash(ctx, b.String())
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

func (n Native) Bash(ctx context.Context, script string) (*runner.Result, error) {
	args := n.bashArgs(script)
	displayArgs := n.bashArgs("<script>")
	if n.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] bash command: %s %s\n", "bash", strings.Join(displayArgs, " "))
	}
	return runner.CombinedOutputDisplay(ctx, "bash "+strings.Join(displayArgs, " "), "bash", args...)
}

func (n Native) BashStream(ctx context.Context, script string, onLine func(string)) (*runner.Result, error) {
	args := n.bashArgs(script)
	displayArgs := n.bashArgs("<script>")
	if n.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] bash command: %s %s\n", "bash", strings.Join(displayArgs, " "))
	}
	return runner.StreamOutputDisplay(ctx, "bash "+strings.Join(displayArgs, " "), "bash", onLine, args...)
}

func (n Native) bashArgs(script string) []string {
	return []string{"--noprofile", "--norc", "-c", script}
}

func Doctor(ctx context.Context) []Check {
	checks := []Check{
		{
			Name:    "OS",
			OK:      runtime.GOOS == "linux",
			Detail:  runtime.GOOS + "/" + runtime.GOARCH,
			Warning: runtime.GOOS != "linux",
		},
	}
	if runtime.GOOS != "linux" {
		return checks
	}

	for _, dep := range []string{"bash", "dpkg-deb", "tar", "xz", "dwarf2json", "rpm2cpio", "cpio", "gzip", "zstd"} {
		depCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		path, err := lookPathWithContext(depCtx, dep)
		cancel()
		if err != nil {
			checks = append(checks, Check{Name: dep, OK: false, Detail: err.Error()})
			continue
		}
		checks = append(checks, Check{Name: dep, OK: true, Detail: path})
	}
	return checks
}

func lookPathWithContext(ctx context.Context, name string) (string, error) {
	type result struct {
		path string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		path, err := exec.LookPath(name)
		ch <- result{path: path, err: err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case out := <-ch:
		if out.err != nil {
			return "", out.err
		}
		return out.path, nil
	}
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
