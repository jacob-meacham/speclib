package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/jacob-meacham/speclib/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "speclib:", err)
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		os.Exit(1)
	}
}
