package main

import (
	"fmt"
	"strings"
)

// completionCommands and completionFlags are the single source of truth for the
// generated shell completions, so they cannot drift from the dispatch table.
var completionCommands = []string{"report", "lint", "diff", "policies", "explain", "keygen", "version", "completion", "help"}

var completionFlags = []string{
	"--manifests", "--regulation", "--config", "--format", "--out", "--cluster-region",
	"--fail-on", "--strict", "--sign-key", "--json", "--values", "--require-region",
	"--require-egress-policy", "--allow-external-name", "--allow-external-ips",
	"--baseline", "--current", "--fail-on-regression", "--kubeconfig", "--context",
}

func runCompletion(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: regionlock completion bash|zsh|fish|powershell")
	}
	s, err := completionScript(args[0])
	if err != nil {
		return err
	}
	fmt.Print(s)
	return nil
}

func completionScript(shell string) (string, error) {
	cmds := strings.Join(completionCommands, " ")
	flags := strings.Join(completionFlags, " ")
	switch shell {
	case "bash":
		return fmt.Sprintf(bashCompletion, cmds, flags), nil
	case "zsh":
		return fmt.Sprintf(zshCompletion, cmds, flags), nil
	case "fish":
		return fishCompletion(), nil
	case "powershell", "pwsh":
		return fmt.Sprintf(powershellCompletion, quoteList(completionCommands)+","+quoteList(completionFlags)), nil
	default:
		return "", fmt.Errorf("unsupported shell %q (want bash|zsh|fish|powershell)", shell)
	}
}

func quoteList(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = "'" + x + "'"
	}
	return strings.Join(q, ",")
}

func fishCompletion() string {
	var b strings.Builder
	b.WriteString("# regionlock fish completion. Install: regionlock completion fish > ~/.config/fish/completions/regionlock.fish\n")
	b.WriteString("complete -c regionlock -f\n")
	b.WriteString("complete -c regionlock -n '__fish_use_subcommand' -a '" + strings.Join(completionCommands, " ") + "'\n")
	for _, f := range completionFlags {
		b.WriteString("complete -c regionlock -l '" + strings.TrimPrefix(f, "--") + "'\n")
	}
	return b.String()
}

const bashCompletion = `# regionlock bash completion. Install: regionlock completion bash > /etc/bash_completion.d/regionlock
_regionlock() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "%s" -- "$cur") )
    return
  fi
  COMPREPLY=( $(compgen -W "%s" -- "$cur") )
}
complete -F _regionlock regionlock
`

const zshCompletion = `#compdef regionlock
# regionlock zsh completion. Install: regionlock completion zsh > "${fpath[1]}/_regionlock"
_regionlock() {
  if (( CURRENT == 2 )); then
    local -a cmds; cmds=(%s)
    _describe 'command' cmds
  else
    _arguments '*:flag:(%s)'
  fi
}
_regionlock "$@"
`

const powershellCompletion = `# regionlock PowerShell completion. Install: regionlock completion powershell | Out-String | Invoke-Expression
Register-ArgumentCompleter -Native -CommandName regionlock -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)
  @(%s) | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
  }
}
`
