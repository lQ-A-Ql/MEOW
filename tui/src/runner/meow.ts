import { runCommand, type RunCommandResult } from "./process"

export type MeowRunOptions = {
  meowPath: string
  args: string[]
  cwd?: string
  signal?: AbortSignal
  onStdout?: (chunk: string) => void
  onStderr?: (chunk: string) => void
}

export type MeowJSONResult = {
  data: unknown
  result: RunCommandResult
}

export function buildMeowJSONArgs(args: string[]): string[] {
  return ["--json", ...args]
}

export async function runMeowJSON(options: MeowRunOptions): Promise<MeowJSONResult> {
  const result = await runCommand(options.meowPath, buildMeowJSONArgs(options.args), {
    cwd: options.cwd,
    signal: options.signal,
    onStdout: options.onStdout,
    onStderr: options.onStderr,
  })

  const parsed = parseJSON(result.stdout)

  if (result.code !== 0) {
    throw new MeowCommandError(options.args, result, parsed)
  }

  if (parsed === undefined) {
    throw new Error(`meow returned invalid JSON for: ${options.args.join(" ")}\nstdout=${result.stdout}\nstderr=${result.stderr}`)
  }

  return { data: parsed, result }
}

export function runDoctor(options: Omit<MeowRunOptions, "args">) {
  return runMeowJSON({ ...options, args: ["doctor"] })
}

export function runParseBannerFile(options: Omit<MeowRunOptions, "args"> & { bannerFile: string; noRemoteSymbols?: boolean }) {
  const args = ["parse", "--banner-file", options.bannerFile]
  if (options.noRemoteSymbols ?? true) {
    args.push("--no-remote-symbols")
  }

  return runMeowJSON({ ...options, args })
}

export function runBuildDryRun(options: Omit<MeowRunOptions, "args"> & { bannerFile: string; noRemoteSymbols?: boolean }) {
  const args = ["build", "--dry-run", "--banner-file", options.bannerFile]
  if (options.noRemoteSymbols ?? true) {
    args.push("--no-remote-symbols")
  }

  return runMeowJSON({ ...options, args })
}

export function runVerify(options: Omit<MeowRunOptions, "args"> & { memPath: string; symbolsPath: string }) {
  return runMeowJSON({
    ...options,
    args: ["verify", "--mem", options.memPath, "--symbols", options.symbolsPath],
  })
}

export function runCacheList(options: Omit<MeowRunOptions, "args">) {
  return runMeowJSON({ ...options, args: ["cache", "list"] })
}

export class MeowCommandError extends Error {
  constructor(
    public readonly args: string[],
    public readonly result: RunCommandResult,
    public readonly parsedStdout: unknown,
  ) {
    super(
      `meow failed: ${args.join(" ")}\n` +
        `code=${result.code}\n` +
        `stdout=${result.stdout}\n` +
        `stderr=${result.stderr}\n` +
        `parsed=${JSON.stringify(parsedStdout, null, 2)}`,
    )
    this.name = "MeowCommandError"
  }
}

function parseJSON(content: string): unknown | undefined {
  try {
    return JSON.parse(content)
  } catch {
    return undefined
  }
}
