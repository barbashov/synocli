package main

import (
	"fmt"
	"os"

	"synocli/internal/apperr"
)

func main() {
	root := newRootCmd(os.Stdin, os.Stdout, os.Stderr)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(apperr.ExitCode(err))
	}
}
