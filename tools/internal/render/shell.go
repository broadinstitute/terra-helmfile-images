package render

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
	"strings"
)

/*
Wrapper around exec.Command to support interface-based mocking for unit testing

https://joshrendek.com/2014/06/go-lang-mocking-exec-dot-command-using-interfaces/
*/
type ShellRunner interface {
	Run(cmd Command) error
}

type ShellError struct {
	Command Command
	Err     error
}

func (e *ShellError) Error() string {
	cmd := e.Command.PrettyFormat()
	if exitErr, ok := e.Err.(*exec.ExitError); ok {
		// Command exited non-zero
		return fmt.Sprintf("Command exited with status %d: %q", exitErr.ExitCode(), cmd)
	} else {
		// Command failed to start for some reason
		return fmt.Sprintf("Command %q failed to start: %v", cmd, e.Err)
	}
}

type Command struct {
	Prog string
	Args []string
	Dir  string
}

/* Convert command into simple string for easy inspection. Eg.
&Command{
  Prog: []string{"echo"},
  Args: []string{"foo", "bar", "baz"},
  Dir:  "/tmp",
}
->
"echo foo bar baz"
*/
func (c *Command) PrettyFormat() string {
	return strings.Join(append([]string{c.Prog}, c.Args...), " ")
}

type RealRunner struct{}

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
