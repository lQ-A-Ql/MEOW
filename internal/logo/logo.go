package logo

import "fmt"

var logoLines = []string{
	"███╗   ███╗███████╗ ██████╗ ██╗    ██╗    ██╗",
	"████╗ ████║██╔════╝██╔═══██╗██║    ██║    ██║",
	"██╔████╔██║█████╗  ██║   ██║██║ █╗ ██║    ██║",
	"██║╚██╔╝██║██╔══╝  ██║   ██║██║███╗██║    ╚═╝",
	"██║ ╚═╝ ██║███████╗╚██████╔╝╚███╔███╔╝    ██╗",
	"╚═╝     ╚═╝╚══════╝ ╚═════╝  ╚══╝╚══╝     ╚═╝",
}

func gradientRGB(col, totalCols int) (int, int, int) {
	t := float64(col) / float64(totalCols-1)
	switch {
	case t < 0.33:
		s := t / 0.33
		return lerp(0, 120, s), lerp(200, 80, s), lerp(255, 255, s)
	case t < 0.66:
		s := (t - 0.33) / 0.33
		return lerp(120, 255, s), lerp(80, 0, s), lerp(255, 200, s)
	default:
		s := (t - 0.66) / 0.34
		return lerp(255, 255, s), lerp(0, 150, s), lerp(200, 50, s)
	}
}

func lerp(a, b int, t float64) int {
	return a + int(float64(b-a)*t)
}

func isBlockChar(c rune) bool {
	return c != ' '
}

func printGradientLine(line string) {
	runes := []rune(line)
	totalCols := len(runes)
	for col, ch := range runes {
		if ch == ' ' || !isBlockChar(ch) {
			fmt.Print(string(ch))
			continue
		}
		r, g, b := gradientRGB(col, totalCols)
		fmt.Printf("\033[38;2;%d;%d;%dm%c\033[0m", r, g, b, ch)
	}
	fmt.Println()
}

func Print() {
	for _, line := range logoLines {
		printGradientLine(line)
	}
	printGradientLine("=^..^=__/  meow~ Vol3 Linux Symbol Builder v0.1.0")
	fmt.Println()
}
