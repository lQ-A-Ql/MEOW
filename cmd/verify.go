package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"meow/internal/log"
	"meow/internal/volatility"
)

var (
	verifyMem     string
	verifySymbols string
	verifyVolPath string
	verifyJSON    bool
)

func init() {
	fs := Register("verify", "verify symbol directory with Volatility 3", runVerify)
	fs.StringVar(&verifyMem, "mem", "", "memory image path")
	fs.StringVar(&verifySymbols, "symbols", filepathDefaultSymbols(), "symbols directory")
	fs.StringVar(&verifyVolPath, "vol", "vol", "Volatility 3 command path")
	fs.BoolVar(&log.Verbose, "verbose", false, "verbose logging")
	fs.BoolVar(&verifyJSON, "json", false, "output JSON")
}

func runVerify(args []string) {
	jsonMode := verifyJSON || JSONFlag
	applyVerifyConfigDefaults(jsonMode)
	if verifyMem == "" {
		verifyFail(jsonMode, "", fmt.Errorf("missing --mem"))
	}

	if _, err := exec.LookPath(verifyVolPath); err != nil {
		verifyFail(jsonMode, "", fmt.Errorf("Volatility 3 not found (%s); confirm vol is in PATH or pass --vol", verifyVolPath))
	}

	output, err := volatility.Verify(context.Background(), verifyVolPath, verifyMem, verifySymbols)
	if err != nil {
		verifyFail(jsonMode, output, err)
	}

	if jsonMode {
		data, _ := json.MarshalIndent(map[string]any{
			"success": true,
			"output":  output,
		}, "", "  ")
		fmt.Println(string(data))
		return
	}
	log.Success("Volatility 3 loaded symbol table successfully.")
	log.Success("linux.pslist.PsList executed successfully.")
}

func applyVerifyConfigDefaults(jsonMode bool) {
	cfg, err := readOrDefaultConfig()
	if err != nil {
		if !jsonMode {
			log.Warn("failed to read config defaults: %v", err)
		}
		return
	}
	if !flagWasSet(Commands["verify"].Flags, "vol") {
		verifyVolPath = cfg.VolatilityPath
	}
}

func verifyFail(jsonMode bool, output string, err error) {
	if jsonMode {
		data, _ := json.MarshalIndent(map[string]any{
			"success": false,
			"error":   err.Error(),
			"output":  output,
		}, "", "  ")
		fmt.Println(string(data))
	} else {
		log.Error("symbol verification failed: %v", err)
		fmt.Fprintln(os.Stderr, "Possible causes:")
		fmt.Fprintln(os.Stderr, "  1. symbols/linux directory layout is wrong.")
		fmt.Fprintln(os.Stderr, "  2. json.xz is corrupt.")
		fmt.Fprintln(os.Stderr, "  3. banner does not match.")
		fmt.Fprintln(os.Stderr, "  4. Volatility 3 cache is stale.")
		fmt.Fprintln(os.Stderr, "Suggested command: meow cache clear")
	}
	os.Exit(1)
}

func filepathDefaultSymbols() string {
	return "./symbols"
}
