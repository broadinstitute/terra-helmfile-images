package shellmock

import (
	"fmt"
	"github.com/stretchr/testify/mock"
	"io"
)

// Decorates testify's Call struct with additional methods for simulating stdout / stderr output from a mocked command
type Call struct {
	mockStdout string
	mockStderr string
	*mock.Call
}

// Configures the mock command to write the given data to stdout
func (c *Call) WithStdout(output string) *Call {
	c.mockStdout = output
	return c
}

// Configures the mock command to write the given data to stder
func (c *Call) WithStderr(output string) *Call {
	c.mockStderr = output
	return c
}

// write mock output to arguments
func (c *Call) writeMockOutput(args mock.Arguments) error {
	if err := writeMockOutputToStream(args, 1, c.mockStdout); err != nil {
		return err
	}
	if err := writeMockOutputToStream(args, 2, c.mockStderr); err != nil {
		return err
	}
	return nil
}

func writeMockOutputToStream(args mock.Arguments, index int, mockOutput string) error {
	arg := args.Get(index)
	stream, ok := arg.(io.Writer)
	if stream != nil && !ok {
		return fmt.Errorf("shellmock.Call: type assertion failed: expected io.Writer, got %v", arg)
	}
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