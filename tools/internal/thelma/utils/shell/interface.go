package shell

import (
	"fmt"
	"github.com/rs/zerolog"
	"io"
	"strings"
)

//
// Runner is an interface for running shell commands. It exists to
// support mocking shell commands in unit tests.
//
// https://joshrendek.com/2014/06/go-lang-mocking-exec-dot-command-using-interfaces/
//
type Runner interface {
	// Run runs a command, streaming stdout and stderr to the log at debug level.
	Run(cmd Command) error

	// Capture runs a Command, streaming stdout and stderr to the given writers.
	// An error is returned if the command exits non-zero.
	// If you're only interested in stdout, pass in nil for stderr (and vice versa)
	RunWith(cmd Command, opts RunOptions) error
}

// Command encapsulates a shell command
type Command struct {
	Prog        string   // Prog Main CLI program to execute
	Args        []string // Args Arguments to pass to program
	Env         []string // Env List of environment variables, eg []string{ "FOO=BAR", "BAZ=QUUX" }, to set when executing
	Dir         string   // Dir Directory where command should be run
	PristineEnv bool     // PristineEnv When true, set only supplied Env vars without inheriting current process's env vars
}

// PrettyFormat converts a command into a simple string for easy inspection. Eg.
// &Command{
//   Prog: []string{"echo"},
//   Args: []string{"foo", "bar", "baz"},
//   Dir:  "/tmp",
//   Env:  []string{"A=B", "C=D"}
// }
// ->
// "A=B C=D echo foo bar baz"
func (c Command) PrettyFormat() string {
	// TODO shellquote arguments for better readability
	var a []string
	a = append(a, c.Env...)
	a = append(a, c.Prog)
	a = append(a, c.Args...)
	return strings.Join(a, " ")
}

// Options for a RunWith() invocation
type RunOptions struct {
	// optional logger to use for logging this command
	Logger *zerolog.Logger
	// optional level at which command should be logged
	LogLevel *zerolog.Level
	// optional writer where stdout should be written
	Stdout io.Writer
	// optional writer where stderr should be written
	Stderr io.Writer
}

// Error is a generic error that is returned in situations other than the command failing.
// (eg. if the Command's Directory does not exist)
type Error struct {
	Command Command // the command that generated this error e
	err     error   // underlying error returned by exec package
}

// Error generates a user-friendly error message
func (e *Error) Error() string {
	cmd := e.Command.PrettyFormat()
	return fmt.Sprintf("Command %q failed to start: %v", cmd, e.err)
}

// ExitError is returned when a command fails
type ExitError struct {
	Command  Command // the command that generated this error
	ExitCode int     // exit code of command
	Stderr   string  // stderr output
}

// Error generates a user-friendly error message for failed shell commands
func (e *ExitError) Error() string {
	cmd := e.Command.PrettyFormat()
	msg := fmt.Sprintf("Command %q exited with status %d", cmd, e.ExitCode)
	stderr := e.Stderr
	// Add stderr output if any was generated
	if len(stderr) > 0 {
		msg = fmt.Sprintf("%s:\n%s", msg, stderr)
	}

	return msg
}
