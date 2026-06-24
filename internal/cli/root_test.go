package cli

import (
	"testing"
)

func TestRootRegistersArchitectureGlobalFlags(t *testing.T) {
	root := (&app{}).rootCommand()
	for _, name := range []string{"port", "auto-port", "allow-unsupported", "baud", "timeout", "format", "verbose", "yes"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("missing persistent flag %q", name)
		}
	}
}
