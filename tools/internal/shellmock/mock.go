package shellmock

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
	"strings"
	"testing"
)

//
// The shellmock package makes it easy to mock shell commands in unit tests with testify/mock.
//
// It contains a mock implementation of the shell.Runner interface, called MockRunner.
// Unlike testify's out-of-the-box mock implementation, MockRunner can verify that shell
// commands are run in a specific order.
//

// FailureMode: what to do when there's an order verification failure (panic or fail the test)
type FailureMode int

const (
	Panic    FailureMode = iota // Panic when an order mismatch is found
	FailTest                    // FailTest will fail the test when an order mismatch is found
)

// Options for a MockRunner
type Options struct {
	VerifyOrder bool        // VerifyOrder If true, verify commands are run in the order they were declared
	FailureMode FailureMode // FailureMode How to handle order verification failures
}

// MockRunner is an implementation of Runner interface for use with testify/mock.
type MockRunner struct {
	mock             mock.Mock // mock underlying testify mock object
	options          Options
	expectedCommands []shell.Command
	receivedCounter  int
}

// DefaultMockRunner returns a new mock runner instance with default settings
func DefaultMockRunner() *MockRunner {
	options := Options{
		VerifyOrder: true,
		FailureMode: FailTest,
	}
	return NewMockRunner(options)
}

// NewMockRunner constructor for MockRunner
func NewMockRunner(options Options) *MockRunner {
	m := new(MockRunner)
	m.options = options
	return m
}

// Run Instead of executing the command, logs an info message and registers the call with testify mock
func (m *MockRunner) Run(cmd shell.Command) error {
	log.Info().Msgf("[MockRunner] Run called: %q\n", cmd.PrettyFormat())
	args := m.mock.Called(cmd)

	// Return error if one was added to the mocked call
	if len(args) > 0 {
		return args.Error(0)
	} else {
		return nil
	}
}

// ExpectCmd Updates mock runner with an expectation for a given command.
// Unlike with vanilla testify mocks, an error is raised if a command is invoked out of order.
func (m *MockRunner) ExpectCmd(t *testing.T, cmd shell.Command) *mock.Call {
	// Register the mock with the testify mock object
	call := m.mock.On("Run", cmd)

	// If we aren't verifying call order, then there's nothing to do!
	if !m.options.VerifyOrder {
		return call
	}

	order := len(m.expectedCommands)
	m.expectedCommands = append(m.expectedCommands, cmd)

	return call.Run(func(args mock.Arguments) {
		if m.receivedCounter != order {
			if m.receivedCounter < len(m.expectedCommands) {
				err := fmt.Errorf(
					"Command received out of order (%d instead of %d). Expected:\n%q\nReceived:\n%q",
					m.receivedCounter,
					order,
					m.expectedCommands[m.receivedCounter].PrettyFormat(),
					cmd.PrettyFormat(),
				)

				if m.options.FailureMode == FailTest {
					t.Error(err)
				} else {
					panic(err)
				}
			}
		}

		m.receivedCounter++
	})
}

// ExpectCmdS is a convenience function for generating a Command from a string and expecting it
func (m *MockRunner) ExpectCmdS(t *testing.T, str string) *mock.Call {
	cmd := CmdFmt(str)
	return m.ExpectCmd(t, cmd)
}

// ExpectCmdFmt is a convenience function combining CmdFmt and ExpectCmd
func (m *MockRunner) ExpectCmdFmt(t *testing.T, fmt string, a ...interface{}) *mock.Call {
	cmd := CmdFmt(fmt, a...)
	return m.ExpectCmd(t, cmd)
}

// AssertExpectations delegates to testify/mock's AssertExpectations
func (m *MockRunner) AssertExpectations(t *testing.T) bool {
	return m.mock.AssertExpectations(t)
}

// CmdFmt is a convenience function for creating a Command, given a format string
// and arguments.
//
// Eg. CmdFmt("HOME=%s FOO=%s ls -l %d", "/root", "BAR", "/tmp")
// ->
// Command{
//   Env: []string{"HOME=/root", "FOO=BAR"},
//   Prog: "ls",
//   Args: []string{"-l", "/tmp"},
//   ...
// }
//
// Note: CmdFmt is NOT smart about shell quoting and escaping. I.e.
// "echo hello\\ world" will be parsed as "echo", "hello\\", "world" instead
// of "echo", "hello world". Similarly, "echo 'hello world'" will be parsed as
// "echo", "'hello", "world'". If you need to test arguments with whitespace or other
// special characters, create a shell.Command manually.
//
func CmdFmt(format string, a ...interface{}) shell.Command {
	formatted := fmt.Sprintf(format, a...)
	tokens := strings.Fields(formatted)

	// count number of leading NAME=VALUE environment var pairs preceding `helmfile` command
	var i int
	for i = 0; i < len(tokens); i++ {
		if !strings.Contains(tokens[i], "=") {
			// if this is not a NAME=VALUE pair, exit
			break
		}
	}

	numEnvVars := i
	progIndex := i
	numArgs := len(tokens) - (numEnvVars + 1)

	cmd := shell.Command{}

	if numEnvVars > 0 {
		cmd.Env = tokens[0:numEnvVars]
	}
	if progIndex < len(tokens) {
		cmd.Prog = tokens[progIndex]
	}
	if numArgs > 0 {
		cmd.Args = tokens[progIndex+1:]
	}

	return cmd
}
