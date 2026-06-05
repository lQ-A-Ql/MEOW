export type RunCommandOptions = {
  cwd?: string
  signal?: AbortSignal
  onStdout?: (chunk: string) => void
  onStderr?: (chunk: string) => void
}

export type RunCommandResult = {
  command: string
  args: string[]
  code: number | null
  stdout: string
  stderr: string
  durationMs: number
}

export async function runCommand(
  command: string,
  args: string[],
  options: RunCommandOptions = {},
): Promise<RunCommandResult> {
  const start = Date.now()

  const proc = Bun.spawn([command, ...args], {
    cwd: options.cwd,
    stdin: "ignore",
    stdout: "pipe",
    stderr: "pipe",
    signal: options.signal,
  })

  const stdoutChunks: string[] = []
  const stderrChunks: string[] = []

  const stdoutTask = readStream(proc.stdout, (chunk) => {
    stdoutChunks.push(chunk)
    options.onStdout?.(chunk)
  })

  const stderrTask = readStream(proc.stderr, (chunk) => {
    stderrChunks.push(chunk)
    options.onStderr?.(chunk)
  })

  const code = await proc.exited
  await Promise.all([stdoutTask, stderrTask])

  return {
    command,
    args,
    code,
    stdout: stdoutChunks.join(""),
    stderr: stderrChunks.join(""),
    durationMs: Date.now() - start,
  }
}

async function readStream(stream: ReadableStream<Uint8Array>, onChunk: (chunk: string) => void) {
  const reader = stream.getReader()
  const decoder = new TextDecoder()

  try {
    while (true) {
      const { value, done } = await reader.read()
      if (done) {
        break
      }

      if (value) {
        onChunk(decoder.decode(value, { stream: true }))
      }
    }

    const tail = decoder.decode()
    if (tail) {
      onChunk(tail)
    }
  } finally {
    reader.releaseLock()
  }
}
