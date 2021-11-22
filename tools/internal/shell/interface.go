package shell

import (
	"fmt"
	"io"
	"os/exec"
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
	Capture(cmd Command, stdout io.Writer, stderr io.Writer) error
}

// Error represents an error encountered running a shell command
type Error struct {
	Command Command // the command that generated this error
	Err     error   // underlying error returned by exec package
	ErrOut  string  // any output the command sent to stderr
}

// Command encapsulates a shell command
type Command struct {
	Prog        string   // Prog Main CLI program to execute
	Args        []string // Args Arguments to pass to program
	Env         []string // Env List of environment variables, eg []string{ "FOO=BAR", "BAZ=QUUX" }, to set when executing
	Dir         string   // Dir Directory where command should be run
	PristineEnv bool     // PristineEnv When true, set only supplied Env vars without inheriting current process's env vars
}

// Error generates a user-friendly error message for failed shell commands
func (e *Error) Error() string {
	cmd := e.Command.PrettyFormat()
	if exitErr, ok := e.Err.(*exec.ExitError); ok {
		// Command exited non-zero
		msg := fmt.Sprintf("Command %q exited with status %d", cmd, exitErr.ExitCode())

		// Add stderr output if any was generated
		if len(e.ErrOut) > 0 {
			msg = fmt.Sprintf("%s:\n%s", msg, e.ErrOut)
		}

		return msg
	}

	// Command failed to start for some reason
	return fmt.Sprintf("Command %q failed to start: %v", cmd, e.Err)
}

// PrettyFormat converts command into a simple string for easy inspection. Eg.
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
