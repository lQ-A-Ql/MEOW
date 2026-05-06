package log

import (
	"fmt"
	"os"
)

var Verbose bool

func Success(format string, args ...interface{}) {
	fmt.Printf("[+] "+format+"\n", args...)
}

func Info(format string, args ...interface{}) {
	fmt.Printf("[*] "+format+"\n", args...)
}

func Warn(format string, args ...interface{}) {
	fmt.Printf("[!] "+format+"\n", args...)
}

func NonFatal(format string, args ...interface{}) {
	fmt.Printf("[-] "+format+"\n", args...)
}

func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", args...)
}

func Fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", args...)
	os.Exit(1)
}

func Debug(format string, args ...interface{}) {
	if Verbose {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}
