import { runCommand } from "./process"

export type VolRunOptions = {
  volPath: string
  memPath: string
  symbolsPath?: string
  plugin: string
  extraArgs?: string[]
  cwd?: string
  signal?: AbortSignal
  onStdout?: (chunk: string) => void
  onStderr?: (chunk: string) => void
}

export function buildVolArgs(options: Pick<VolRunOptions, "memPath" | "symbolsPath" | "plugin" | "extraArgs">): string[] {
  const args = ["-f", options.memPath]

  if (options.symbolsPath && options.symbolsPath.trim() !== "") {
    args.push("-s", options.symbolsPath)
  }

  args.push(options.plugin)

  if (options.extraArgs?.length) {
    args.push(...options.extraArgs)
  }

  return args
}

export function runVolPlugin(options: VolRunOptions) {
  return runCommand(options.volPath, buildVolArgs(options), {
    cwd: options.cwd,
    signal: options.signal,
    onStdout: options.onStdout,
    onStderr: options.onStderr,
  })
}

export function extractBanner(options: Omit<VolRunOptions, "plugin" | "symbolsPath" | "extraArgs">) {
  return runVolPlugin({
    ...options,
    plugin: "banners.Banners",
  })
}

export function runPsList(options: Omit<VolRunOptions, "plugin" | "extraArgs">) {
  return runVolPlugin({
    ...options,
    plugin: "linux.pslist.PsList",
  })
}
