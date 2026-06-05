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

export type ImageInfo = {
  banner?: string
  os?: string
  kernel?: string
  arch?: string
  distro?: string
  packageVersion?: string
}

export type AppState = {
  meowPath: string
  volPath: string

  memPath: string
  symbolsPath: string
  bannerFile: string
  outDir: string
  cacheDir: string
  plugin: string

  imageInfo: ImageInfo | null

  logs: LogEntry[]
  nextLogId: number
  runningTask?: RunningTask

  commandInput: string
  inputFocused: boolean

  lastParseResult?: unknown
  lastBuildResult?: unknown
  lastVolOutput?: string
  lastDoctorResult?: unknown
  lastCacheResult?: unknown
}
