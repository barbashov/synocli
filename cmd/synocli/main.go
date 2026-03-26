package main

import (
	"errors"
	"os"

	"synocli/internal/apperr"
)

func main() {
	root := newRootCmd(os.Stdin, os.Stdout, os.Stderr)
	if err := root.Execute(); err != nil {
		var handled *jsonOutputHandledError
		if !errors.As(err, &handled) {
			printError(os.Stderr, err)
		}
		os.Exit(apperr.ExitCode(err))
	}
}
