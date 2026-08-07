package agent

import (
	"fmt"
	"sort"
	"strings"
)

// Adapter describes one supported coding agent: the binary speclib launches
// and the permission args its headless print mode gets by default. The table
// below is the seam for supporting agents beyond claude; the manifest's
// [agent] section selects an adapter and can replace its permission args.
type Adapter struct {
	Name               string
	Bin                string
	DefaultPermissions []string
}

var adapters = map[string]Adapter{
	"claude": {
		Name:               "claude",
		Bin:                "claude",
		DefaultPermissions: []string{"--allowedTools", "Write,Edit,Bash"},
	},
}

// Lookup resolves an [agent].command value to an adapter. An empty name
// selects the default (claude); an unknown name errors, naming the supported
// adapters.
func Lookup(name string) (Adapter, error) {
	if name == "" {
		name = "claude"
	}
	a, ok := adapters[name]
	if !ok {
		names := make([]string, 0, len(adapters))
		for n := range adapters {
			names = append(names, n)
		}
		sort.Strings(names)
		return Adapter{}, fmt.Errorf("unknown agent %q: supported agents are %s", name, strings.Join(names, ", "))
	}
	return a, nil
}

// HeadlessArgs builds the print-mode argv (minus the binary): the generation
// prompt, streamed JSON output, and the permission args — permissions
// verbatim when non-empty (the manifest override), else the adapter's
// defaults.
func (a Adapter) HeadlessArgs(prompt string, permissions []string) []string {
	if len(permissions) == 0 {
		permissions = a.DefaultPermissions
	}
	args := []string{"-p", prompt, "--output-format", "stream-json", "--verbose"}
	return append(args, permissions...)
}

// InteractiveArgs builds the argv that opens the agent's own interactive UI
// with instructions as the initial prompt.
func (a Adapter) InteractiveArgs(instructions string) []string {
	return []string{instructions}
}
