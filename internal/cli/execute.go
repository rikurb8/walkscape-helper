package cli

import (
	"bytes"
	"io"

	"walkscape-helper/internal/output"
)

func Execute(args []string, stdin io.Reader) (stdout, stderr string, exitCode int) {
	cmd := NewRootCmd()
	outBuf := bytes.NewBuffer(nil)
	errBuf := bytes.NewBuffer(nil)
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	if stdin != nil {
		cmd.SetIn(stdin)
	}
	cmd.SetArgs(args)

	err := cmd.Execute()
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	return outBuf.String(), errBuf.String(), output.ExitCode(err)
}
