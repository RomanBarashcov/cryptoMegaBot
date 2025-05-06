package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	verbose        = flag.Bool("v", false, "verbose output")
	short          = flag.Bool("short", false, "run only short tests")
	timeout        = flag.Duration("timeout", 5*time.Minute, "test timeout")
	testRegexp     = flag.String("run", "", "run only tests matching the regular expression")
	backtestOnly   = flag.Bool("backtest", false, "run only backtesting tests")
	indicatorsOnly = flag.Bool("indicators", false, "run only indicator tests")
)

func main() {
	flag.Parse()

	// Build test command
	args := []string{"test"}

	// Add verbose flag if requested
	if *verbose {
		args = append(args, "-v")
	}

	// Add short flag if requested
	if *short {
		args = append(args, "-short")
	}

	// Add timeout
	args = append(args, fmt.Sprintf("-timeout=%s", timeout.String()))

	// Add test regexp if provided
	if *testRegexp != "" {
		args = append(args, fmt.Sprintf("-run=%s", *testRegexp))
	}

	// Add package specifier
	switch {
	case *backtestOnly:
		args = append(args, "./internal/strategy/backtesting/...", "./internal/strategy/strategies/...")
	case *indicatorsOnly:
		args = append(args, "./internal/strategy/indicators/...")
	default:
		args = append(args, "./...")
	}

	// Create command
	cmd := exec.Command("go", args...)

	// Set environment variables for tests
	env := os.Environ()
	env = append(env, "TEST_ENV=true")
	cmd.Env = env

	// Redirect output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run tests
	fmt.Printf("Running tests with args: %s\n", strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Printf("Error running tests: %v\n", err)
		os.Exit(1)
	}
}
