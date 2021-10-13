package shellmock

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock/matchers"
	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
	"io"
	"os"
	"testing"
)

//
// The shellmock package makes it easy to mock shell commands in unit tests with testify/mock.
//
// See example_test.go for example usage.
//
// Shellmock contains a mock implementation of the shell.Runner interface, called MockRunner.
// Unlike testify's out-of-the-box mock implementation, MockRunner can verify that shell
// commands are run in a specific order.
//

// CmdDumpStyle how to style commands when they are printed to the console
type CmdDumpStyle int

// Default prints the command using "%v"
// Pretty formats commands using PrettyFormat
// Spew uses the spew library to spew the entire struct
const (
	Default CmdDumpStyle = iota
	Pretty
	Spew
)

// Options for a MockRunner
type Options struct {
	VerifyOrder bool         // VerifyOrder If true, verify commands are run in the order they were declared
	DumpStyle   CmdDumpStyle // DumpStyle how to style the dump
}

type expectedCommand struct {
	cmd  shell.Command
	call *mock.Call
}

// MockRunner is an implementation of Runner interface for use with testify/mock.
type MockRunner struct {
	options          Options
	expectedCommands []expectedCommand
	runCounter       int
	t                *testing.T
	mock.Mock
}

// DefaultMockRunner returns a new mock runner instance with default settings
func DefaultMockRunner() *MockRunner {
	options := Options{
		VerifyOrder: true,
	}
	return NewMockRunner(options)
}

// NewMockRunner constructor for MockRunner
func NewMockRunner(options Options) *MockRunner {
	m := new(MockRunner)
	m.options = options
	return m
}

// RunWithArgs delegates to Run
func (m *MockRunner) RunWithArgs(prog string, args ...string) error {
	return m.Run(shell.Command{
		Prog: prog,
		Args: args,
	})
}

// Run Instead of executing the command, logs an info message and registers the call with testify mock
func (m *MockRunner) Run(cmd shell.Command) error {
	log.Info().Msgf("[MockRunner] Run called: %q\n", cmd.PrettyFormat())
	args := m.Mock.Called(cmd)
	if len(args) > 0 {
		return args.Error(0)
	}
	return nil
}

// ExpectCmd sets an expectation for a command that should be run. It accepts either a shell.Command or a CmdMatcher
func (m *MockRunner) ExpectCmd(cmdOrMatcher interface{}) *mock.Call {
	var call *mock.Call
	var cmd shell.Command

	switch c := cmdOrMatcher.(type) {
	case *matchers.CmdMatcher:
		cmd = c.AsCmd()
		call = m.Mock.On("Run", mock.MatchedBy(func(actual shell.Command) bool {
			return c.Matches(actual)
		}))
	case shell.Command:
		cmd = c
		call = m.Mock.On("Run", c)
	default:
		m.panicOrFailNow(fmt.Errorf("ExpectCmd only supports shell.Command or matchers.CmdMatcher arguments, got %v (type %T)", cmdOrMatcher, cmdOrMatcher))
		return nil
	}

	order := len(m.expectedCommands)
	m.expectedCommands = append(m.expectedCommands, expectedCommand{cmd: cmd, call: call})

	return call.Run(func(args mock.Arguments) {
		if m.options.VerifyOrder {
			if m.runCounter != order { // this command is out of order
				if m.runCounter < len(m.expectedCommands) { // we have remaining expectations
					err := fmt.Errorf(
						"Command received out of order (%d instead of %d). Expected:\n%v\nReceived:\n%v",
						m.runCounter,
						order,
						m.expectedCommands[m.runCounter].cmd,
						cmd,
					)

					m.panicOrFailNow(err)
				}
			}
		}

		m.runCounter++
	})
}

// Test decorates Testify's mock.Mock#Test() function by adding a cleanup hook to the test object
// that dumps the set of expected command matchers to stderr in the event of a test failure.
// This is useful because most command matchers are functions and so Testify can't generate
// a pretty diff for them; you end up with:
//   (shell.Command={...}) not matched by func(Command) bool
//
func (m *MockRunner) Test(t *testing.T) {
	m.t = t
	t.Cleanup(func() {
		if t.Failed() {
			if err := m.dumpExpectedCmds(os.Stderr); err != nil {
				t.Error(err)
			}
		}
	})
	m.Mock.Test(t)
}

func (m *MockRunner) dumpExpectedCmds(w io.Writer) error {
	if _, err := fmt.Fprint(w, "\n\nExpected commands:\n\n"); err != nil {
		return err
	}
	for i, ec := range m.expectedCommands {
		if err := m.dumpExpectedCmd(w, i, ec); err != nil {
			return err
		}
	}

	return nil
}

func (m *MockRunner) dumpExpectedCmd(w io.Writer, index int, expected expectedCommand) error {
	cmd := expected.cmd
	switch m.options.DumpStyle {
	case Default:
		if _, err := fmt.Fprintf(w, "\t%d: %#v\n\n", index, cmd); err != nil {
			return err
		}
	case Pretty:
		if _, err := fmt.Fprintf(w, "\t%d: %s\n\n", index, cmd.PrettyFormat()); err != nil {
			return err
		}
	case Spew:
		if _, err := fmt.Fprintf(w, "\t%d: %s\n\n", index, cmd.PrettyFormat()); err != nil {
			return err
		}

		scs := spew.ConfigState{
			Indent:                  "\t",
			DisableCapacities:       true,
			DisablePointerAddresses: true,
		}

		scs.Fdump(w, cmd)

		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}

		fmt.Println()
	}

	return nil
}

func (m *MockRunner) panicOrFailNow(err error) {
	if m.t == nil {
		panic(err)
	}
	m.t.Error(err)
	m.t.FailNow()
}
