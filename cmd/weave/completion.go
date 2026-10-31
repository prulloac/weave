package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

var completionCommands = func() []string {
	commands := make([]string, 0, len(commandHelpText)+1)
	for command := range commandHelpText {
		commands = append(commands, command)
	}
	commands = append(commands, "help")
	sort.Strings(commands)
	return commands
}()

func runCompletion(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printCompletionHelp(stdout)
		return 0
	}
	if len(args) != 1 {
		return usageError(stdout, stderr, false, "completion requires bash, zsh or fish")
	}
	commands := strings.Join(completionCommands, " ")
	switch args[0] {
	case "bash":
		fmt.Fprintf(stdout, `_weave_completion() {
	local commands="%s"
	COMPREPLY=( $(compgen -W "$commands" -- "${COMP_WORDS[1]}") )
}
complete -F _weave_completion weave
`, commands)
	case "zsh":
		fmt.Fprintf(stdout, `#compdef weave
_weave() {
	local -a commands
	commands=(%s)
	_describe 'command' commands
}
_weave "$@"
`, commands)
	case "fish":
		fmt.Fprintf(stdout, `complete -c weave -f -n "__fish_use_subcommand" -a "%s"
`, commands)
	default:
		return usageError(stdout, stderr, false, "completion requires bash, zsh or fish")
	}
	return 0
}

func printCompletionHelp(writer io.Writer) {
	fmt.Fprint(writer, `weave completion — print a shell completion script

usage:
  weave completion bash|zsh|fish

examples:
  weave completion bash > ~/.local/share/bash-completion/completions/weave
  weave completion zsh > "${fpath[1]}/_weave"
  weave completion fish > ~/.config/fish/completions/weave.fish
`)
}
