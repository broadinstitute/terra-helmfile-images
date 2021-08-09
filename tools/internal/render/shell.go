package render

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
	"strings"
)

//
// ShellRunner is an interface for running shell commands. It exists to
// support mocking shell commands in unit tests.
//
// https://joshrendek.com/2014/06/go-lang-mocking-exec-dot-command-using-interfaces/
//
type ShellRunner interface {
	Run(cmd Command) error
}

// ShellError represents an error encountered running a shel command
type ShellError struct {
	Command Command
	Err     error
}

// Error generates a user-friendly error message for failed shell commands
func (e *ShellError) Error() string {
	cmd := e.Command.PrettyFormat()
	if exitErr, ok := e.Err.(*exec.ExitError); ok {
		// Command exited non-zero
		return fmt.Sprintf("Command %q exited with status %d", cmd, exitErr.ExitCode())
	}

	// Command failed to start for some reason
	return fmt.Sprintf("Command %q failed to start: %v", cmd, e.Err)
}

// Command encapsulates a shell command
type Command struct {
	Prog string   // Main CLI program to execute
	Args []string // Arguments to pass to program
	Dir  string   // Directory where command should be run
}

// PrettyFormat converts command into a simple string for easy inspection. Eg.
// &Command{
//   Prog: []string{"echo"},
//   Args: []string{"foo", "bar", "baz"},
//   Dir:  "/tmp",
// }
// ->
// "echo foo bar baz"
func (c *Command) PrettyFormat() string {
	// TODO shellquote arguments for better readability
	return strings.Join(append([]string{c.Prog}, c.Args...), " ")
}

// RealRunner is an implementation of the Runner API that actually executes shell commands
// (contrast with MockRunner)
type RealRunner struct{}

// Run runs a Command, returning an error if the command exits non-zero
func (r *RealRunner) Run(cmd Command) error {
	execCmd := exec.Command(cmd.Prog, cmd.Args...)
	execCmd.Dir = cmd.Dir

	// TODO - would be nice to capture out/err and stream to debug log, to cut down on noise
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	log.Info().Msgf("Executing: %v", execCmd)
	err := execCmd.Run()
	if err != nil {
		log.Error().Msgf("Command failed: %v", err)
		return &ShellError{Command: cmd, Err: err}
	}

	return nil
}
