package main

import (
	"fmt"
	"os"

	"github.com/jacob-meacham/speclib/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "speclib:", err)
		os.Exit(1)
	}
}
