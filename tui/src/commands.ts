import type { AppState } from "./types"

export type ParsedCommand = {
  command: string
  args: string
}

export function parseCommand(input: string): ParsedCommand {
  const trimmed = input.trim()
  if (!trimmed.startsWith("/")) {
    return { command: "unknown", args: trimmed }
  }

  const spaceIndex = trimmed.indexOf(" ")
  if (spaceIndex === -1) {
    return { command: trimmed.slice(1), args: "" }
  }

  return {
    command: trimmed.slice(1, spaceIndex),
    args: trimmed.slice(spaceIndex + 1).trim(),
  }
}

export type CommandResult = {
  state: AppState
  output?: string
  action?: "run" | "banner" | "build" | "verify" | "clear" | "help"
}

export function executeCommand(state: AppState, input: string): CommandResult {
  const { command, args } = parseCommand(input)

  switch (command) {
    case "symbol": {
      if (!args) {
        return { state, output: "用法: /symbol <符号表路径>\n示例: /symbol ./symbols" }
      }
      return {
        state: { ...state, symbolsPath: args },
        output: `✓ 符号表路径已设置: ${args}`,
      }
    }

    case "plugin": {
      if (!args) {
        return { state, output: "用法: /plugin <插件名>\n示例: /plugin linux.pslist.PsList" }
      }
      return {
        state: { ...state, plugin: args },
        output: `✓ 当前插件已设置: ${args}`,
      }
    }

    case "mem": {
      if (!args) {
        return { state, output: "用法: /mem <内存镜像路径>\n示例: /mem ./memory.raw" }
      }
      return {
        state: { ...state, memPath: args },
        output: `✓ 内存镜像路径已设置: ${args}`,
      }
    }

    case "run": {
      if (!state.memPath) {
        return { state, output: "✗ 请先设置内存镜像: /mem <path>" }
      }
      if (!state.plugin) {
        return { state, output: "✗ 请先设置插件: /plugin <name>" }
      }
      return { state, action: "run" }
    }

    case "banner": {
      if (!state.memPath) {
        return { state, output: "✗ 请先设置内存镜像: /mem <path>" }
      }
      return { state, action: "banner" }
    }

    case "build": {
      return { state, action: "build" }
    }

    case "verify": {
      if (!state.memPath) {
        return { state, output: "✗ 请先设置内存镜像: /mem <path>" }
      }
      return { state, action: "verify" }
    }

    case "clear": {
      return { state: { ...state, logs: [], nextLogId: 1 }, action: "clear" }
    }

    case "help": {
      return {
        state,
        output: [
          "可用命令:",
          "",
          "  /symbol <path>    设置符号表路径",
          "  /plugin <name>    设置当前 Volatility 插件",
          "  /mem <path>       设置内存镜像路径",
          "",
          "  /run              执行当前插件",
          "  /banner           提取内核 banner",
          "  /build            运行 meow build --dry-run",
          "  /verify           验证符号表",
          "",
          "  /clear            清空输出日志",
          "  /help             显示此帮助",
          "",
          "快捷键: i 聚焦输入 | Esc 取消聚焦 | q 退出 | x 取消任务",
        ].join("\n"),
      }
    }

    default:
      return {
        state,
        output: `✗ 未知命令: /${command}\n输入 /help 查看可用命令`,
      }
  }
}
