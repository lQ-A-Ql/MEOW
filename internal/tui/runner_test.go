package tui

import "runtime"

func testCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-Command", "Write-Output 'out'; Write-Error 'err'; exit 7"}
	}
	return "sh", []string{"-c", "printf '%s\n' out; printf '%s\n' err >&2; exit 7"}
}

func sleepCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-Command", "Start-Sleep -Milliseconds 500"}
	}
	return "sh", []string{"-c", "sleep 1"}
}
