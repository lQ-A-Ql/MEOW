import type { AppState, ScreenName } from "./types"

export type ScreenDefinition = {
  key: string
  name: ScreenName
  label: string
  description: string
}

export const screens: ScreenDefinition[] = [
  {
    key: "1",
    name: "dashboard",
    label: "Dashboard",
    description: "Environment overview and quick actions",
  },
  {
    key: "2",
    name: "parse",
    label: "Parse",
    description: "Parse a Linux banner or banner fixture",
  },
  {
    key: "3",
    name: "build",
    label: "Build",
    description: "Generate or dry-run Volatility symbols",
  },
  {
    key: "4",
    name: "volatility",
    label: "Volatility",
    description: "Run Volatility 3 plugins against a memory image",
  },
  {
    key: "5",
    name: "workflow",
    label: "Workflow",
    description: "Extract banner, build symbols, then verify",
  },
  {
    key: "6",
    name: "cache",
    label: "Cache",
    description: "Inspect MEOW cache state",
  },
  {
    key: "7",
    name: "logs",
    label: "Logs",
    description: "Full command and application log",
  },
]

export function findScreenByKey(key: string): ScreenDefinition | undefined {
  return screens.find((screen) => screen.key === key)
}

export function findScreen(name: ScreenName): ScreenDefinition {
  return screens.find((screen) => screen.name === name) ?? screens[0]
}

export function getNextScreen(current: ScreenName, offset: number): ScreenName {
  const currentIndex = screens.findIndex((screen) => screen.name === current)
  const safeIndex = currentIndex >= 0 ? currentIndex : 0
  const nextIndex = (safeIndex + offset + screens.length) % screens.length
  return screens[nextIndex].name
}

export function getScreenLines(state: AppState): string[] {
  switch (state.activeScreen) {
    case "dashboard":
      return [
        "MEOW Linux symbol helper TUI",
        "",
        `meow path: ${state.meowPath}`,
        `vol path:  ${state.volPath}`,
        `symbols:   ${state.symbolsPath}`,
        `output:    ${state.outDir}`,
        "",
        "Actions:",
        "  r          run doctor via ../meow --json doctor",
        "  x          cancel running command",
        "",
        "Navigation:",
        "  1-7        switch screens",
        "  [ / ]      previous / next screen",
        "  d          dashboard",
        "  l          logs",
        "  q / Esc    quit",
        "",
        ...previewResult("Last doctor result", state.lastDoctorResult),
      ]
    case "parse":
      return [
        "Parse banner",
        "",
        `banner file: ${state.bannerFile}`,
        "",
        "Action:",
        "  r          ../meow --json parse --banner-file <file> --no-remote-symbols",
        "",
        ...previewResult("Last parse result", state.lastParseResult),
      ]
    case "build":
      return [
        "Build symbols dry-run",
        "",
        `banner file: ${state.bannerFile}`,
        `output dir:  ${state.outDir}`,
        "",
        "Action:",
        "  r          ../meow --json build --dry-run --banner-file <file> --no-remote-symbols",
        "",
        ...previewResult("Last build result", state.lastBuildResult),
      ]
    case "volatility":
      return [
        "Volatility 3",
        "",
        `memory image: ${state.memPath || "<not set>"}`,
        `symbols:      ${state.symbolsPath}`,
        `plugin:       ${state.plugin}`,
        "",
        "Action:",
        "  r          vol -f <mem> -s ./symbols <plugin>",
        "",
        "The MVP keeps subprocess calls as argv arrays; no shell command strings are built.",
        "",
        ...previewText("Last vol output", state.lastVolOutput),
      ]
    case "workflow":
      return [
        "Workflow",
        "",
        "Action:",
        "  r          run MVP workflow with the current memory image/banner fixture",
        "",
        "Sequence:",
        "  1. vol -f <mem> banners.Banners when memory image is set",
        "  2. ../meow --json build --dry-run --banner-file <banner>",
        "  3. ../meow --json verify --mem <mem> --symbols ./symbols when memory image is set",
        "",
        "All steps run inside WSL/Linux; no wsl.exe bridge is used.",
      ]
    case "cache":
      return [
        "Cache",
        "",
        `cache dir override: ${state.cacheDir || "<default $HOME/.meow/cache>"}`,
        "",
        "Action:",
        "  r          ../meow --json cache list",
      ]
    case "logs":
      return state.logs.length === 0
        ? ["No logs yet."]
        : state.logs.map((entry) => `[${entry.time}] ${entry.level.toUpperCase()} ${entry.message}`)
  }
}

function previewResult(title: string, value: unknown): string[] {
  if (value === undefined) {
    return [`${title}: <none>`]
  }

  return [`${title}:`, ...JSON.stringify(value, null, 2).split("\n").slice(0, 12)]
}

function previewText(title: string, value: string | undefined): string[] {
  if (!value) {
    return [`${title}: <none>`]
  }

  return [`${title}:`, ...value.split("\n").filter(Boolean).slice(0, 12)]
}
