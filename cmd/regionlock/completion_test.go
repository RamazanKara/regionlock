package main

import (
	"strings"
	"testing"
)

// TestCompletionScriptsCoverAllCommands guards against the hand-written shell
// completions drifting from the command list when a subcommand is added.
func TestCompletionScriptsCoverAllCommands(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		s, err := completionScript(sh)
		if err != nil {
			t.Fatalf("%s: %v", sh, err)
		}
		for _, c := range completionCommands {
			if !strings.Contains(s, c) {
				t.Errorf("%s completion is missing command %q", sh, c)
			}
		}
	}
	if _, err := completionScript("tcsh"); err == nil {
		t.Error("expected an error for an unsupported shell")
	}
}
