package main

import "fmt"

func cmdStatus(args []string) {
	repoRoot, err := findRepoFromState()
	if err != nil {
		fatal(err.Error())
	}

	// Watch mode
	if len(args) > 0 && (args[0] == "--watch" || args[0] == "-w") {
		runStatusWatch(repoRoot)
		return
	}

	// Static mode
	fmt.Println(renderStatusView(repoRoot, 120))
}
