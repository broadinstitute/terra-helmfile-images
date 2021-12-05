package shell

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/testutils"
	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
	"io"
	"os"
	"strings"
	"sync"
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

// options for a MockRunner
type MockOptions struct {
	VerifyOrder   bool         // VerifyOrder If true, verify commands are run in the order they were declared
	DumpStyle     CmdDumpStyle // DumpStyle how to style the dump
	IgnoreEnvVars []string     // Array of environment variable names to strip when matching shell.Command arguments
	IgnoreDir     bool         // If true, ignore Dir field of shell.Command arguments
}

// MockRunner is an implementation of Runner interface for use with testify/mock.
type MockRunner struct {
	options          MockOptions
	ignoreEnvVars    map[string]struct{}
	expectedCommands []*expectedCommand
	runCounter       int
	t                *testing.T
	mutex            sync.Mutex
	mock.Mock
}

type expectedCommand struct {
	cmd        Command
	matchCount int
}

// DefaultMockRunner returns a new mock runner instance with default settings
func DefaultMockRunner() *MockRunner {
	options := MockOptions{
		VerifyOrder: true,
	}
	return NewMockRunner(options)
}

// NewMockRunner constructor for MockRunner
func NewMockRunner(options MockOptions) *MockRunner {
	m := new(MockRunner)
	m.options = options

	// convert ignoreEnvVars from array to map for fast lookups
	m.ignoreEnvVars = make(map[string]struct{}, len(m.options.IgnoreEnvVars))
	for _, name := range m.options.IgnoreEnvVars {
		m.ignoreEnvVars[name] = struct{}{}
	}

	return m
}

// Convenience function to build a shell.Command from a format string and arguments
//
// Eg. CmdFromFmt("HOME=%s FOO=BAR ls -al %s", "/tmp", "Documents")
// ->
// Command{
//   Env: []string{"HOME=/tmp", "FOO=BAR"},
//   Prog: "ls",
//   Args: []string{"-al", "Documents},
//   Dir: ""
// }
func CmdFromFmt(fmt string, args ...interface{}) Command {
	tokens := testutils.Args(fmt, args...)

	return CmdFromArgs(tokens...)
}

// Convenience function to build a shell.Command from a list of arguments
//
// Eg. CmdFromArgs("FOO=BAR", "ls", "-al", ".")
// ->
// Command{
//   Env: []string{"FOO=BAR"},
//   Prog: "ls",
//   Args: []string{"-al", "."},
//   Dir: ""
// }
func CmdFromArgs(args ...string) Command {
	// count number of leading NAME=VALUE environment var pairs preceding command
	var i int
	for i = 0; i < len(args); i++ {
		if !strings.Contains(args[i], "=") {
			// if this is not a NAME=VALUE pair, exit
			break
		}
	}

	numEnvVars := i
	progIndex := i
	numArgs := len(args) - (numEnvVars + 1)

	var cmd Command

	if numEnvVars > 0 {
		cmd.Env = args[0:numEnvVars]
	}
	if progIndex < len(args) {
		cmd.Prog = args[progIndex]
	}
	if numArgs > 0 {
		cmd.Args = args[progIndex+1:]
	}

	return cmd
}

// Run Instead of executing the command, logs an info message and registers the call with testify mock
func (m *MockRunner) Run(cmd Command) error {
	return m.RunWith(cmd, RunOptions{})
}

// Capture Instead of executing the command, log an info message and register the call with testify mock
func (m *MockRunner) RunWith(cmd Command, opts RunOptions) error {
	log.Info().Msgf("[MockRunner] Command: %q\n", cmd.PrettyFormat())

	// Remove ignored attributes
	cmd = m.applyIgnores(cmd)

	// we synchronize Run calls on the mock because testify mock isn't safe for concurrent access, and neither are our
	// order verification callback hooks
	m.mutex.Lock()
	defer m.mutex.Unlock()

	args := m.Mock.Called(cmd, opts)
	if len(args) > 0 {
		return args.Error(0)
	}
	return nil
}

// ExpectCmd sets an expectation for a command that should be run.
func (m *MockRunner) ExpectCmd(cmd Command) *Call {
	cmd = m.applyIgnores(cmd)

	mockCall := m.Mock.On("RunWith", cmd, mock.AnythingOfType("RunOptions"))
	callWrapper := &Call{
		command: cmd,
		Call:    mockCall,
	}

	order := len(m.expectedCommands)
	expected := &expectedCommand{cmd: cmd}
	m.expectedCommands = append(m.expectedCommands, expected)

	callWrapper.Run(func(args mock.Arguments) {
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

		if err := callWrapper.writeMockOutput(args); err != nil {
			m.panicOrFailNow(err)
		}

		expected.matchCount++
		m.runCounter++
	}).Once()

	return callWrapper
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

func (m *MockRunner) applyIgnores(cmd Command) Command {
	if m.options.IgnoreDir {
		cmd.Dir = ""
	}

	if len(m.ignoreEnvVars) == 0 {
		return cmd
	}

	var env []string
	for _, pair := range cmd.Env {
		tokens := strings.SplitN(pair, "=", 2)
		name := tokens[0]

		// if env var is not in ignore list, keep it
		if _, exists := m.ignoreEnvVars[name]; !exists {
			env = append(env, pair)
		}
	}
	cmd.Env = env

	return cmd
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

func (m *MockRunner) dumpExpectedCmd(w io.Writer, index int, expected *expectedCommand) error {
	cmd := expected.cmd
	switch m.options.DumpStyle {
	case Default:
		if _, err := fmt.Fprintf(w, "\t%d (%d matches):\n\t%#v\n\n", index, expected.matchCount, cmd); err != nil {
			return err
		}
	case Pretty:
		if _, err := fmt.Fprintf(w, "\t%d (%d matches): %s\n\n", index, expected.matchCount, cmd.PrettyFormat()); err != nil {
			return err
		}
	case Spew:
		if _, err := fmt.Fprintf(w, "\t%d (%d matches): %s\n\n", index, expected.matchCount, cmd.PrettyFormat()); err != nil {
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
