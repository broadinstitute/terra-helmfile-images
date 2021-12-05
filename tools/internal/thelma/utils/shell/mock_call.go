package shell

import (
	"fmt"
	"github.com/stretchr/testify/mock"
	"io"
)

// Decorates testify's Call struct with additional methods for simulating stdout / stderr output from a mocked command
type Call struct {
	mockStdout string
	mockStderr string
	command    Command
	*mock.Call
}

// Configures the mock command to exit with a non-zero exit code
func (c *Call) ExitsNonZero() *Call {
	return c.Exits(1)
}

// Configures the mock command to exit with the given exit status
func (c *Call) Exits(exitCode int) *Call {
	if exitCode != 0 {
		err := &ExitError{
			Command:  c.command,
			ExitCode: exitCode,
			Stderr:   c.mockStderr,
		}
		c.Return(err)
	}
	return c
}

// Configures the mock command to fail with a non-ExitError error
func (c *Call) Fails(err error) *Call {
	c.Return(&Error{
		Command: c.command,
		err:     err,
	})
	return c
}

// Configures the mock command to write the given data to stdout
func (c *Call) WithStdout(output string) *Call {
	c.mockStdout = output
	return c
}

// Configures the mock command to write the given data to stderr
func (c *Call) WithStderr(output string) *Call {
	c.mockStderr = output
	return c
}

// write mock output to arguments
func (c *Call) writeMockOutput(args mock.Arguments) error {
	runOpts, ok := args.Get(1).(RunOptions)
	if !ok {
		panic(fmt.Errorf("shellmock.Call: type assertion failed: expected RunOpts, got: %v", args.Get(1)))
	}
	if err := writeMockOutputToStream(runOpts.Stdout, c.mockStdout); err != nil {
		return err
	}
	if err := writeMockOutputToStream(runOpts.Stderr, c.mockStderr); err != nil {
		return err
	}
	return nil
}

func writeMockOutputToStream(stream io.Writer, mockOutput string) error {
	if stream == nil {
		// nothing to write to
		return nil
	}
	if mockOutput == "" {
		// no mock output to write
		return nil
	}
	_, err := stream.Write([]byte(mockOutput))
	return err
}
