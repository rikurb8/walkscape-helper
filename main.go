package main

import (
	"errors"
	"fmt"
	"os"

	"walkscape-helper/internal/cli"
	"walkscape-helper/internal/output"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		if !output.IsHandled(err) {
			var appErr *output.AppError
			if errors.As(err, &appErr) {
				fmt.Fprintln(os.Stderr, appErr.Message)
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		os.Exit(output.ExitCode(err))
	}
}
