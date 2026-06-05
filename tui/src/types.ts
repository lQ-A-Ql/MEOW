export type ScreenName =
  | "dashboard"
  | "parse"
  | "build"
  | "volatility"
  | "workflow"
  | "cache"
  | "logs"

export type LogLevel = "info" | "success" | "warn" | "error" | "stdout" | "stderr"

export type LogEntry = {
  id: number
  level: LogLevel
  message: string
  time: string
}

export type RunningTask = {
  id: string
  title: string
  abortController: AbortController
}

export type AppState = {
  activeScreen: ScreenName
  meowPath: string
  volPath: string

  memPath: string
  symbolsPath: string
  bannerFile: string
  outDir: string
  cacheDir: string
  plugin: string

  logs: LogEntry[]
  nextLogId: number
  runningTask?: RunningTask

  lastParseResult?: unknown
  lastBuildResult?: unknown
  lastVolOutput?: string
  lastDoctorResult?: unknown
  lastCacheResult?: unknown
}
