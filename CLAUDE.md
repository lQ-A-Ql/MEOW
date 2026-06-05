# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- Build the CLI: `go build -o meow .`
- Run all tests: `go test ./...`
- Run a single package test: `go test ./internal/resolver -run TestGenerateCandidates -v`
- Run one named test in `cmd`: `go test ./cmd -run TestFormatDownloadProgressUsesStablePixelCat -v`
- Lint with the repository config: `golangci-lint run ./...`
- Format Go code before linting: `gofmt -w $(git ls-files '*.go')`; run `goimports -w $(git ls-files '*.go')` if imports changed (`.golangci.yml` enforces goimports with local prefix `meow`).

Smoke commands from CI/README:

- Parse a fixture without remote symbol lookup: `go run . --json parse --banner-file ./testdata/banners/ubuntu_5.4.0_163.txt --no-remote-symbols`
- Linux-only dry-run build smoke: `go build -o meow . && ./meow --json build --dry-run --banner-file ./testdata/banners/ubuntu_5.4.0_163.txt --no-remote-symbols`
- Linux-only RPM-family dry-run smoke: `./meow --json build --dry-run --banner-file ./testdata/banners/centos_4.18.0_513.txt --no-remote-symbols`
- Check runtime dependencies on Linux: `./meow doctor`
- Verify generated symbols with Volatility 3: `./meow verify --mem ./memdump.mem --symbols ./symbols`

`build` and `doctor` intentionally fail on non-Linux hosts (`当前版本仅支持 Linux 原生运行`). `parse`, `cache`, and `config` are safe to run while developing on Windows, but CI executes on Ubuntu.

## Runtime dependencies and paths

The current implementation is Linux-native and no longer uses `wsl.exe` or the legacy `--backend` / `--wsl-distro` flags. For symbol generation, Linux needs `bash`, `dpkg-deb`, `tar`, `xz`, `dwarf2json`, `rpm2cpio`, `cpio`, `gzip`, and `zstd`; `verify` and `build --mem` also need Volatility 3 as `vol` or via `--vol`.

Default user state lives under `$HOME/.meow`:

- `$HOME/.meow/config.json`
- `$HOME/.meow/symbol-sources.txt`
- `$HOME/.meow/cache/`

Generated symbols default to `./symbols/linux`. When using Volatility 3, pass the parent directory with `-s ./symbols`, not `./symbols/linux`.

## High-level architecture

`main.go` only handles early `--json` detection for suppressing the colored logo, then calls `cmd.Execute()`. The `cmd` package implements a small custom CLI registry around Go's `flag` package; each command registers itself from `init()`. Global flags (`--json`, `--verbose`) are parsed before the subcommand, and many commands also expose command-local `--json` / `--verbose` flags.

The primary build flow is:

1. `cmd/build.go` accepts input from a pasted/banner-file banner, `--mem`, manual `--kernel` + `--pkgver`, `--debug-package`/legacy `--ddeb`, `--debug-package-url`/legacy `--ddeb-url`, `--repo-url`, or `--vmlinux`.
2. `internal/volatility` extracts a Linux banner from memory images by running `vol -f <mem> banners.Banners`; `verify` runs `linux.banners.Banners` and `linux.pslist.PsList` with `-s <symbols>`.
3. `internal/banner` parses `KernelInfo`. Ubuntu and Debian get package versions; RPM-family banners get partial distro/kernel/arch info for manual or repo-assisted flows.
4. `internal/symbolsources` checks exact banner matches in remote ISF indexes from `symbol-sources.txt` first. The default source is Abyss-W4tcher's public Volatility 3 symbols repository. A remote ISF hit downloads `.json.xz` directly and skips debug-package generation.
5. If no remote ISF matches, `internal/resolver` generates package candidates: Ubuntu `.ddeb`, Debian `.deb`, RPM-family names for manual use, or RPM repo lookup via `repodata/repomd.xml` when `--repo-url` is supplied. Probing uses `HEAD` with a Range GET fallback for servers that reject HEAD.
6. `internal/downloader` downloads packages or remote symbols with progress callbacks. `internal/cache` stores downloads and metadata under `$HOME/.meow/cache`, keyed by URL hash, and respects `--force` for re-download/regeneration.
7. `internal/backend.Native` embeds `internal/backend/scripts/debug_package.sh` and `vmlinux.sh`, then runs them through clean `bash --noprofile --norc -c`. The scripts extract `.ddeb`/`.deb`/`.rpm` packages, find or decompress `vmlinux`, run `dwarf2json linux --elf`, compress with `xz`, and move the final `${Distro}_${Kernel}_${PackageVersion}_${Arch}.json.xz` into the output directory.
8. `cmd/progress.go` renders the terminal progress UI from backend marker lines such as `VOLSYM_STAGE=...` and `VOLSYM_EXTRACT_*`. JSON mode must keep stdout pure JSON, so logo and progress output stay disabled there.

## Repository notes

- `README.md` is the current user-facing behavior reference. `PRD.md` still contains older Windows/WSL backend language; treat the code and README as authoritative for current Linux-native behavior.
- The output filename format is centralized in `internal/symbols.FileName`; tests expect it to include distro, kernel, package version, and arch.
- `testdata/banners/` contains the banner fixtures used by smoke tests and parser/resolver tests.
- No `.cursor/rules`, `.cursorrules`, or `.github/copilot-instructions.md` files are present in this repository at the time this file was created.
