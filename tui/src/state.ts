import type { AppState, LogLevel, RunningTask } from "./types"

export type { AppState } from "./types"

export function createInitialState(): AppState {
  return {
    meowPath: "../meow",
    volPath: "vol",

    memPath: "",
    symbolsPath: "./symbols",
    bannerFile: "../testdata/banners/ubuntu_5.4.0_163.txt",
    outDir: "./symbols/linux",
    cacheDir: "",
    plugin: "linux.pslist.PsList",

    imageInfo: null,

    logs: [],
    nextLogId: 1,

    commandInput: "",
    inputFocused: false,
  }
}

export function appendLog(state: AppState, level: LogLevel, message: string): AppState {
  return {
    ...state,
    logs: [
      ...state.logs,
      {
        id: state.nextLogId,
        level,
        message,
        time: new Date().toLocaleTimeString(),
      },
    ].slice(-200),
    nextLogId: state.nextLogId + 1,
  }
}

export function appendChunkLog(state: AppState, level: Extract<LogLevel, "stdout" | "stderr">, chunk: string): AppState {
  const lines = chunk.split("\n").map((line) => line.trimEnd()).filter(Boolean)
  return lines.reduce((next, line) => appendLog(next, level, line), state)
}

export function setRunningTask(state: AppState, runningTask: RunningTask): AppState {
  return { ...state, runningTask }
}

export function clearRunningTask(state: AppState): AppState {
  const { runningTask: _runningTask, ...rest } = state
  return rest
}

export function setCommandInput(state: AppState, commandInput: string): AppState {
  return { ...state, commandInput }
}

export function setInputFocused(state: AppState, inputFocused: boolean): AppState {
  return { ...state, inputFocused }
}

export function setImageInfo(state: AppState, imageInfo: AppState["imageInfo"]): AppState {
  return { ...state, imageInfo }
}
