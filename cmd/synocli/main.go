package main

import (
	"os"

	"synocli/internal/apperr"
	"synocli/internal/cli"
)

func main() {
	if err := cli.Execute(os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(apperr.ExitCode(err))
	}
}
