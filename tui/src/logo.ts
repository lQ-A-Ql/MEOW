import { bold, fg, StyledText } from "@opentui/core"
import type { TextChunk } from "@opentui/core"

// Gradient colors: pink → purple → blue → cyan
const COLORS = ["#FF6B9D", "#E879A8", "#D088B3", "#B896BE", "#A0A5C9", "#88B4D4", "#70C3DF", "#58D2EA", "#40E0F5"]

function chunk(text: string, color?: string): TextChunk {
  return color ? fg(color)(text) : fg("#E5E7EB")(text)
}

function gradientLine(line: string, startColor: number): StyledText {
  const chunks = line.split("").map((ch, i) => {
    if (ch === " ") return chunk(" ")
    const colorIdx = (startColor + i) % COLORS.length
    return chunk(ch, COLORS[colorIdx])
  })
  return new StyledText(chunks)
}

const LOGO_LINES = [
  " ███╗   ███╗ ███████╗  ██████╗ ██╗    ██╗",
  " ████╗ ████║ ██╔════╝ ██╔═══██╗██║    ██║",
  " ██╔████╔██║ █████╗   ██║   ██║██║ █╗ ██║",
  " ██║╚██╔╝██║ ██╔══╝   ██║   ██║██║███╗██║",
  " ██║ ╚═╝ ██║ ███████╗ ╚██████╔╝╚███╔███╔╝",
  " ╚═╝     ╚═╝ ╚══════╝  ╚═════╝  ╚══╝╚══╝",
]

export function renderLogo(): StyledText {
  const lines = LOGO_LINES.map((line, i) => gradientLine(line, i * 2))
  const allChunks: TextChunk[] = []
  for (const line of lines) {
    allChunks.push(...line.chunks)
    allChunks.push(chunk("\n"))
  }
  allChunks.push(chunk(" ".repeat(20)))
  allChunks.push(bold(fg("#E879A8")("Linux")))
  allChunks.push(fg("#888")(" Memory Forensics Toolkit"))
  return new StyledText(allChunks)
}

export function getLogoLineCount(): number {
  return LOGO_LINES.length + 1
}
