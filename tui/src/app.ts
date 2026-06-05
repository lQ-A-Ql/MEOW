import { Box, ScrollBox, Text, Input, createCliRenderer, t, bold, fg, type KeyEvent, type StyledText } from "@opentui/core"
import { InputRenderableEvents } from "@opentui/core"

import {
  appendChunkLog,
  appendLog,
  clearRunningTask,
  createInitialState,
  setCommandInput,
  setInputFocused,
  setImageInfo,
  setRunningTask,
} from "./state"
import type { AppState, LogLevel } from "./types"
import { renderLogo, getLogoLineCount } from "./logo"
import { pluginCategories } from "./plugins"
import { executeCommand } from "./commands"
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
    let state = appendLog(createInitialState(), "info", "TUI started — 输入 /help 查看命令")
    let isShuttingDown = false
    let inputRenderable: ReturnType<typeof Input> | null = null

    const redraw = () => {
      render(renderer, state, inputRenderable)
    }

    const setState = (next: AppState) => {
      state = next
      redraw()
    }

    const shutdown = () => {
      if (isShuttingDown) return
      isShuttingDown = true
      state.runningTask?.abortController.abort()
      renderer.destroy()
      process.exit(0)
    }

    const appendStreamLog = (level: StreamLogLevel, chunk: string) => {
      state = appendChunkLog(state, level, chunk)
      redraw()
    }

    const handleCommand = async (input: string) => {
      const result = executeCommand(state, input)
      state = result.state
      if (result.output) {
        state = appendLog(state, "info", result.output)
      }
      if (result.action) {
        await runAction(result.action)
      }
      state = setCommandInput(state, "")
      redraw()
    }

    const runAction = async (action: string) => {
      if (state.runningTask) {
        state = appendLog(state, "warn", `任务运行中: ${state.runningTask.title}`)
        return
      }

      const actionMap: Record<string, () => Promise<AppState>> = {
        run: async () => {
          const result = await runVolPlugin({
            volPath: state.volPath,
            memPath: state.memPath,
            symbolsPath: state.symbolsPath,
            plugin: state.plugin,
            signal: abortController.signal,
            onStdout: (c) => appendStreamLog("stdout", c),
            onStderr: (c) => appendStreamLog("stderr", c),
          })
          if (result.code !== 0) throw new Error(`vol failed with code ${result.code}\n${result.stderr}`)
          return { ...state, lastVolOutput: result.stdout }
        },
        banner: async () => {
          const result = await extractBanner({
            volPath: state.volPath,
            memPath: state.memPath,
            signal: abortController.signal,
            onStdout: (c) => appendStreamLog("stdout", c),
            onStderr: (c) => appendStreamLog("stderr", c),
          })
          if (result.code !== 0) throw new Error(`banner extraction failed with code ${result.code}\n${result.stderr}`)
          return { ...state, lastVolOutput: result.stdout }
        },
        build: async () => {
          const { data } = await runBuildDryRun({
            meowPath: state.meowPath,
            bannerFile: state.bannerFile,
            signal: abortController.signal,
            onStdout: (c) => appendStreamLog("stdout", c),
            onStderr: (c) => appendStreamLog("stderr", c),
          })
          return { ...state, lastBuildResult: data }
        },
        verify: async () => {
          const { data } = await runVerify({
            meowPath: state.meowPath,
            memPath: state.memPath,
            symbolsPath: state.symbolsPath,
            signal: abortController.signal,
            onStdout: (c) => appendStreamLog("stdout", c),
            onStderr: (c) => appendStreamLog("stderr", c),
          })
          return { ...state, lastVerifyResult: data }
        },
      }

      const run = actionMap[action]
      if (!run) return

      const abortController = new AbortController()
      state = setRunningTask(appendLog(state, "info", `执行: ${action}`), {
        id: String(Date.now()),
        title: action,
        abortController,
      })
      redraw()

      try {
        const next = await run()
        state = appendLog(next, "success", `完成: ${action}`)
      } catch (error) {
        const level = abortController.signal.aborted ? "warn" : "error"
        const message = abortController.signal.aborted ? `已取消: ${action}` : formatError(error)
        state = appendLog(state, level, message)
      } finally {
        state = clearRunningTask(state)
        redraw()
      }
    }

    // Initial render
    render(renderer, state, inputRenderable)

    // Setup input component after first render
    const setupInput = () => {
      inputRenderable = Input({
        id: "cmd-input",
        placeholder: "输入命令... (/help 查看帮助)",
        textColor: "#E5E7EB",
        cursorColor: "#60A5FA",
      })

      inputRenderable.on(InputRenderableEvents.ENTER, (value: string) => {
        if (value.trim()) {
          void handleCommand(value)
        }
        inputRenderable!.value = ""
      })

      inputRenderable.on(InputRenderableEvents.INPUT, (value: string) => {
        state = setCommandInput(state, value)
      })

      redraw()
    }

    setupInput()

    // Keyboard handler
    renderer.keyInput.on("keypress", (key: KeyEvent) => {
      // When input is focused, let Input component handle most keys
      if (state.inputFocused) {
        if (key.name === "escape") {
          state = setInputFocused(state, false)
          redraw()
          return
        }
        // Let Input handle the rest
        return
      }

      // Global keys (input not focused)
      if (key.ctrl && key.name === "c") {
        shutdown()
        return
      }

      if (key.name === "q" || key.name === "escape") {
        shutdown()
        return
      }

      if (key.name === "i" || key.name === ":") {
        state = setInputFocused(state, true)
        redraw()
        inputRenderable?.focus()
        return
      }

      if (key.name === "x" && state.runningTask) {
        state.runningTask.abortController.abort()
        state = appendLog(state, "warn", `取消中: ${state.runningTask.title}`)
        redraw()
        return
      }

      if (key.name === "r") {
        void runAction("run")
        return
      }
    })
  } catch (error) {
    renderer.destroy()
    throw error
  }
}

function render(renderer: Renderer, state: AppState, inputRenderable: ReturnType<typeof Input> | null) {
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
        backgroundColor: "#0F172A",
      },
      renderLogoPanel(),
      renderMainRow(state),
      renderCommandBar(state, inputRenderable),
    ),
  )
}

function renderLogoPanel() {
  return Box(
    {
      height: getLogoLineCount() + 1,
      flexDirection: "column",
      alignItems: "center",
      backgroundColor: "#0F172A",
    },
    Text({ content: renderLogo() }),
  )
}

function renderMainRow(state: AppState) {
  return Box(
    {
      flexGrow: 1,
      flexDirection: "row",
    },
    renderLeftPanel(state),
    renderCenterPanel(state),
    renderRightPanel(state),
  )
}

function renderLeftPanel(state: AppState) {
  const lines: StyledText[] = []

  lines.push(t`${bold(fg("#60A5FA")("镜像信息"))}`)
  lines.push(t` `)

  if (state.memPath) {
    lines.push(t`${fg("#888")("内存镜像:")}`)
    lines.push(t`${fg("#E5E7EB")(`  ${state.memPath}`)}`)
  } else {
    lines.push(t`${fg("#888")("内存镜像:")}`)
    lines.push(t`${fg("#6B7280")("  <未设置>")}`)
  }

  lines.push(t` `)
  lines.push(t`${fg("#888")("符号表:")}`)
  lines.push(t`${fg("#E5E7EB")(`  ${state.symbolsPath}`)}`)

  lines.push(t` `)
  lines.push(t`${fg("#888")("输出目录:")}`)
  lines.push(t`${fg("#E5E7EB")(`  ${state.outDir}`)}`)

  lines.push(t` `)
  lines.push(t`${fg("#888")("当前插件:")}`)
  lines.push(t`${fg("#A78BFA")(`  ${state.plugin}`)}`)

  if (state.imageInfo) {
    lines.push(t` `)
    lines.push(t`${bold(fg("#34D399")("Banner 信息"))}`)
    if (state.imageInfo.distro) lines.push(t`${fg("#D1D5DB")(`  发行版: ${state.imageInfo.distro}`)}`)
    if (state.imageInfo.kernel) lines.push(t`${fg("#D1D5DB")(`  内核:   ${state.imageInfo.kernel}`)}`)
    if (state.imageInfo.arch) lines.push(t`${fg("#D1D5DB")(`  架构:   ${state.imageInfo.arch}`)}`)
    if (state.imageInfo.packageVersion) lines.push(t`${fg("#D1D5DB")(`  版本:   ${state.imageInfo.packageVersion}`)}`)
  }

  lines.push(t` `)
  const task = state.runningTask
    ? bold(fg("#FBBF24")(`⟳ ${state.runningTask.title}`))
    : fg("#6B7280")("空闲")
  lines.push(t`${fg("#888")("状态: ")}${task}`)

  return Box(
    {
      width: "22%",
      borderStyle: "rounded",
      borderColor: "#334155",
      flexDirection: "column",
      padding: 1,
    },
    ...lines.map((line, i) => Text({ id: `left-${i}`, content: line })),
  )
}

function renderCenterPanel(state: AppState) {
  const logLines = state.logs.length === 0
    ? [Text({ id: "center-empty", content: "暂无输出。输入命令或按 r 执行当前插件。", fg: "#6B7280" })]
    : state.logs.map((entry) => {
        const color = getLogColor(entry.level)
        return Text({
          id: `log-${entry.id}`,
          content: `[${entry.time}] ${entry.message}`,
          fg: color,
        })
      })

  return ScrollBox(
    {
      flexGrow: 1,
      borderStyle: "rounded",
      borderColor: "#334155",
      flexDirection: "column",
      padding: 1,
      stickyScroll: true,
      stickyStart: "bottom",
    },
    ...logLines,
  )
}

function renderRightPanel(state: AppState) {
  const lines: ReturnType<typeof Text>[] = []

  lines.push(Text({ id: "rp-title", content: t`${bold(fg("#60A5FA")("插件列表"))}` }))
  lines.push(Text({ id: "rp-hint", content: " /plugin <name> 切换", fg: "#6B7280" }))
  lines.push(Text({ id: "rp-spacer", content: " " }))

  for (const category of pluginCategories) {
    lines.push(Text({ id: `rp-cat-${category.name}`, content: t`${category.icon} ${bold(fg("#FBBF24")(category.name))}` }))
    for (const plugin of category.plugins) {
      const isActive = state.plugin === plugin.name
      if (isActive) {
        lines.push(Text({ id: `rp-${plugin.name}`, content: t`${fg("#34D399")("▸ ")}${bold(fg("#34D399")(plugin.name))}` }))
      } else {
        lines.push(Text({ id: `rp-${plugin.name}`, content: t`${fg("#4B5563")("  ")}${fg("#9CA3AF")(plugin.name)}` }))
      }
      lines.push(Text({ id: `rp-d-${plugin.name}`, content: t`${fg("#6B7280")(`    ${plugin.description}`)}` }))
    }
    lines.push(Text({ id: `rp-spacer-${category.name}`, content: " " }))
  }

  return ScrollBox(
    {
      width: 34,
      borderStyle: "rounded",
      borderColor: "#334155",
      flexDirection: "column",
      padding: 1,
    },
    ...lines,
  )
}

function renderCommandBar(state: AppState, inputRenderable: ReturnType<typeof Input> | null) {
  const borderColor = state.inputFocused ? "#60A5FA" : "#334155"
  const hintText = state.inputFocused ? "Esc 取消" : "i 聚焦 | r 执行 | q 退出"

  if (inputRenderable) {
    return Box(
      {
        height: 3,
        borderStyle: "rounded",
        borderColor,
        flexDirection: "row",
        alignItems: "center",
        padding: 0,
      },
      Text({ content: t`${bold(fg("#60A5FA")(" > "))}` }),
      inputRenderable,
      Text({ content: t`  ${fg("#6B7280")(hintText)}` }),
    )
  }

  // Fallback before input is created
  return Box(
    {
      height: 3,
      borderStyle: "rounded",
      borderColor,
      flexDirection: "row",
      alignItems: "center",
      padding: 0,
    },
    Text({ content: t`${bold(fg("#60A5FA")(" > "))}${fg("#6B7280")("输入命令...  ")}${fg("#6B7280")(hintText)}` }),
  )
}

function getLogColor(level: LogLevel): string {
  switch (level) {
    case "info": return "#D1D5DB"
    case "success": return "#34D399"
    case "warn": return "#FBBF24"
    case "error": return "#EF4444"
    case "stdout": return "#93C5FD"
    case "stderr": return "#FCA5A5"
  }
}

function formatError(error: unknown): string {
  if (error instanceof Error) return error.message
  return String(error)
}
