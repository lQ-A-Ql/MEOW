import { Box, ScrollBox, Text, createCliRenderer, type KeyEvent } from "@opentui/core"

import {
  appendChunkLog,
  appendLog,
  clearRunningTask,
  createInitialState,
  setActiveScreen,
  setRunningTask,
} from "./state"
import type { AppState, LogLevel, ScreenName } from "./types"
import { findScreen, findScreenByKey, getNextScreen, getScreenLines, screens } from "./screens"
import { runBuildDryRun, runCacheList, runDoctor, runParseBannerFile, runVerify } from "./runner/meow"
import { extractBanner, runVolPlugin } from "./runner/vol"

type Renderer = Awaited<ReturnType<typeof createCliRenderer>>
type StreamLogLevel = Extract<LogLevel, "stdout" | "stderr">

const rootId = "meow-tui-app"

export async function startApp() {
  const renderer = await createCliRenderer({
    exitOnCtrlC: false,
    exitSignals: ["SIGTERM", "SIGQUIT", "SIGABRT", "SIGHUP", "SIGBREAK", "SIGPIPE", "SIGBUS"],
  })

  try {
    let state = appendLog(createInitialState(), "info", "TUI started")
    let isShuttingDown = false

    const redraw = () => {
      render(renderer, state)
    }

    const setState = (next: AppState) => {
      state = next
      redraw()
    }

    const shutdown = () => {
      if (isShuttingDown) {
        return
      }
      isShuttingDown = true
      state.runningTask?.abortController.abort()
      renderer.destroy()
      process.exit(0)
    }

    const appendStreamLog = (level: StreamLogLevel, chunk: string) => {
      state = appendChunkLog(state, level, chunk)
      redraw()
    }

    const runCurrentAction = async () => {
      if (state.runningTask) {
        setState(appendLog(state, "warn", `A task is already running: ${state.runningTask.title}`))
        return
      }

      const action = getAction(state.activeScreen)
      const abortController = new AbortController()
      state = setRunningTask(appendLog(state, "info", `Running: ${action.title}`), {
        id: String(Date.now()),
        title: action.title,
        abortController,
      })
      redraw()

      try {
        const next = await action.run({
          getState: () => state,
          setState,
          signal: abortController.signal,
          onStdout: (chunk) => appendStreamLog("stdout", chunk),
          onStderr: (chunk) => appendStreamLog("stderr", chunk),
        })
        state = appendLog(next, "success", `Completed: ${action.title}`)
      } catch (error) {
        const level = abortController.signal.aborted ? "warn" : "error"
        const message = abortController.signal.aborted ? `Cancelled: ${action.title}` : formatError(error)
        state = appendLog(state, level, message)
      } finally {
        state = clearRunningTask(state)
        redraw()
      }
    }

    renderer.keyInput.on("keypress", (key: KeyEvent) => {
      if (key.name === "r") {
        void runCurrentAction()
        return
      }

      if (key.name === "x" && state.runningTask) {
        state.runningTask.abortController.abort()
        setState(appendLog(state, "warn", `Cancelling: ${state.runningTask.title}`))
        return
      }

      const nextState = handleKey(state, key, shutdown)
      if (nextState !== state) {
        state = nextState
        redraw()
      }
    })

    redraw()
  } catch (error) {
    renderer.destroy()
    throw error
  }
}

export function handleKey(state: AppState, key: Pick<KeyEvent, "name" | "ctrl">, shutdown?: () => void): AppState {
  if (key.ctrl && key.name === "c") {
    shutdown?.()
    return state
  }

  if (key.name === "escape" || key.name === "q") {
    shutdown?.()
    return state
  }

  if (key.name === "[" || key.name === "left") {
    return switchScreen(state, getNextScreen(state.activeScreen, -1))
  }

  if (key.name === "]" || key.name === "right") {
    return switchScreen(state, getNextScreen(state.activeScreen, 1))
  }

  if (key.name === "d") {
    return switchScreen(state, "dashboard")
  }

  if (key.name === "l") {
    return switchScreen(state, "logs")
  }

  const screen = findScreenByKey(key.name)
  if (screen) {
    return switchScreen(state, screen.name)
  }

  return state
}

type ActionContext = {
  getState: () => AppState
  setState: (state: AppState) => void
  signal: AbortSignal
  onStdout: (chunk: string) => void
  onStderr: (chunk: string) => void
}

type ScreenAction = {
  title: string
  run: (context: ActionContext) => Promise<AppState>
}

function getAction(screen: ScreenName): ScreenAction {
  switch (screen) {
    case "dashboard":
      return {
        title: "doctor",
        run: async ({ getState, signal, onStdout, onStderr }) => {
          const current = getState()
          const { data } = await runDoctor({
            meowPath: current.meowPath,
            signal,
            onStdout,
            onStderr,
          })
          return { ...getState(), lastDoctorResult: data }
        },
      }
    case "parse":
      return {
        title: "parse banner fixture",
        run: async ({ getState, signal, onStdout, onStderr }) => {
          const current = getState()
          const { data } = await runParseBannerFile({
            meowPath: current.meowPath,
            bannerFile: current.bannerFile,
            signal,
            onStdout,
            onStderr,
          })
          return { ...getState(), lastParseResult: data }
        },
      }
    case "build":
      return {
        title: "build dry-run",
        run: async ({ getState, signal, onStdout, onStderr }) => {
          const current = getState()
          const { data } = await runBuildDryRun({
            meowPath: current.meowPath,
            bannerFile: current.bannerFile,
            signal,
            onStdout,
            onStderr,
          })
          return { ...getState(), lastBuildResult: data }
        },
      }
    case "volatility":
      return {
        title: "volatility plugin",
        run: async ({ getState, signal, onStdout, onStderr }) => {
          const current = getState()
          if (!current.memPath) {
            return appendLog(current, "warn", "Set memPath in state before running Volatility.")
          }

          const result = await runVolPlugin({
            volPath: current.volPath,
            memPath: current.memPath,
            symbolsPath: current.symbolsPath,
            plugin: current.plugin,
            signal,
            onStdout,
            onStderr,
          })
          if (result.code !== 0) {
            throw new Error(`vol failed with code ${result.code}\n${result.stderr}`)
          }
          return { ...getState(), lastVolOutput: result.stdout }
        },
      }
    case "workflow":
      return {
        title: "MVP workflow",
        run: async (context) => runWorkflow(context),
      }
    case "cache":
      return {
        title: "cache list",
        run: async ({ getState, signal, onStdout, onStderr }) => {
          const current = getState()
          const { data } = await runCacheList({
            meowPath: current.meowPath,
            signal,
            onStdout,
            onStderr,
          })
          return { ...getState(), lastCacheResult: data }
        },
      }
    case "logs":
      return {
        title: "clear logs",
        run: async ({ getState }) => ({ ...getState(), logs: [], nextLogId: 1 }),
      }
  }
}

async function runWorkflow({ getState, setState, signal, onStdout, onStderr }: ActionContext): Promise<AppState> {
  let current = getState()

  if (current.memPath) {
    setState(appendLog(current, "info", "Workflow step 1/3: extracting banner with vol"))
    const bannerResult = await extractBanner({
      volPath: current.volPath,
      memPath: current.memPath,
      signal,
      onStdout,
      onStderr,
    })
    if (bannerResult.code !== 0) {
      throw new Error(`banner extraction failed with code ${bannerResult.code}\n${bannerResult.stderr}`)
    }
    current = { ...getState(), lastVolOutput: bannerResult.stdout }
    setState(current)
  } else {
    setState(appendLog(current, "warn", "Workflow skipped vol banner extraction because memPath is not set."))
  }

  current = getState()
  setState(appendLog(current, "info", "Workflow step 2/3: running meow build dry-run"))
  const buildResult = await runBuildDryRun({
    meowPath: current.meowPath,
    bannerFile: current.bannerFile,
    signal,
    onStdout,
    onStderr,
  })
  current = { ...getState(), lastBuildResult: buildResult.data }
  setState(current)

  current = getState()
  if (current.memPath) {
    setState(appendLog(current, "info", "Workflow step 3/3: verifying symbols with meow verify"))
    const verifyResult = await runVerify({
      meowPath: current.meowPath,
      memPath: current.memPath,
      symbolsPath: current.symbolsPath,
      signal,
      onStdout,
      onStderr,
    })
    return { ...getState(), lastDoctorResult: verifyResult.data }
  }

  return appendLog(getState(), "warn", "Workflow skipped verify because memPath is not set.")
}

function switchScreen(state: AppState, activeScreen: AppState["activeScreen"]): AppState {
  if (state.activeScreen === activeScreen) {
    return state
  }

  const next = setActiveScreen(state, activeScreen)
  return appendLog(next, "info", `Switched to ${findScreen(activeScreen).label}`)
}

function render(renderer: Renderer, state: AppState) {
  const existing = renderer.root.getRenderable(rootId)
  if (existing) {
    renderer.root.remove(rootId)
  }

  renderer.root.add(
    Box(
      {
        id: rootId,
        width: "100%",
        height: "100%",
        flexDirection: "column",
        backgroundColor: "#111827",
      },
      renderHeader(state),
      renderBody(state),
      renderLogPreview(state),
      renderStatus(state),
    ),
  )
}

function renderHeader(state: AppState) {
  return Box(
    {
      borderStyle: "rounded",
      borderColor: "#38BDF8",
      title: " MEOW TUI ",
      flexDirection: "column",
      padding: 1,
    },
    Text({ content: "Linux symbol generation + Volatility 3", fg: "#E5E7EB" }),
    Text({ content: renderTabs(state), fg: "#93C5FD" }),
  )
}

function renderBody(state: AppState) {
  const screen = findScreen(state.activeScreen)
  const lines = getScreenLines(state)

  return ScrollBox(
    {
      flexGrow: 1,
      borderStyle: "rounded",
      borderColor: "#4B5563",
      title: ` ${screen.label} `,
      flexDirection: "column",
      padding: 1,
      stickyScroll: state.activeScreen === "logs",
      stickyStart: state.activeScreen === "logs" ? "bottom" : undefined,
    },
    ...lines.map((line, index) =>
      Text({
        id: `body-line-${index}`,
        content: line.length === 0 ? " " : line,
        fg: getLineColor(line),
      }),
    ),
  )
}

function renderLogPreview(state: AppState) {
  const preview = state.logs.slice(-3)
  const lines = preview.length === 0 ? ["No logs yet."] : preview.map((entry) => `[${entry.time}] ${entry.level}: ${entry.message}`)

  return Box(
    {
      height: 7,
      borderStyle: "rounded",
      borderColor: "#374151",
      title: " Recent logs ",
      flexDirection: "column",
      padding: 1,
    },
    ...lines.map((line, index) =>
      Text({
        id: `log-line-${index}`,
        content: line,
        fg: "#D1D5DB",
      }),
    ),
  )
}

function renderStatus(state: AppState) {
  const task = state.runningTask ? `running: ${state.runningTask.title}` : "idle"

  return Box(
    {
      height: 3,
      backgroundColor: "#0F172A",
      borderStyle: "single",
      borderColor: "#334155",
      padding: 0,
    },
    Text({
      content: ` ${findScreen(state.activeScreen).description} | ${task} | r run | x cancel | q/Esc quit | 1-7 tabs`,
      fg: "#E2E8F0",
    }),
  )
}

function renderTabs(state: AppState): string {
  return screens
    .map((screen) => {
      const label = `${screen.key}:${screen.label}`
      return screen.name === state.activeScreen ? `[${label}]` : ` ${label} `
    })
    .join(" ")
}

function getLineColor(line: string): string {
  if (line.startsWith("Action") || line.startsWith("Actions") || line.startsWith("Sequence") || line.startsWith("Planned")) {
    return "#FACC15"
  }

  const command = line.trim()
  if (command.startsWith("../meow") || command.startsWith("vol") || command.startsWith("r")) {
    return "#86EFAC"
  }

  if (line.includes("no wsl.exe") || line.includes("argv arrays")) {
    return "#FCA5A5"
  }

  return "#E5E7EB"
}

function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }

  return String(error)
}
