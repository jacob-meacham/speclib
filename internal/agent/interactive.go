package agent

import (
	"fmt"
	"io"
	"os/exec"
)

// LaunchInteractive opens the adapter's own interactive UI with instructions
// as the initial prompt, wiring the given stdio through. When the caller
// passes *os.File values (the real CLI passes the process's stdin/stdout/
// stderr), the child inherits the terminal file descriptors directly, so the
// agent's full-screen UI, permission prompts, and questions all work.
func (a Adapter) LaunchInteractive(instructions string, stdin io.Reader, stdout, stderr io.Writer) error {
	if _, err := exec.LookPath(a.Bin); err != nil {
		return fmt.Errorf("%s not found on PATH — install it, or use `speclib sync --headless`", a.Bin)
	}
	c := exec.Command(a.Bin, a.InteractiveArgs(instructions)...)
	c.Stdin, c.Stdout, c.Stderr = stdin, stdout, stderr
	return c.Run()
}
