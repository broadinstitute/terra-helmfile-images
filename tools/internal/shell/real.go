package shell

import (
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"os/exec"
)

// RealRunner is an implementation of the Runner interface that actually executes shell commands
type RealRunner struct{}

// NewRealRunner constructs a new RealRunner
func NewRealRunner() *RealRunner {
	return &RealRunner{}
}

// Run runs a Command, returning an error if the command exits non-zero
func (r *RealRunner) Run(cmd Command) error {
	execCmd := exec.Command(cmd.Prog, cmd.Args...)
	execCmd.Dir = cmd.Dir

	if !cmd.PristineEnv {
		execCmd.Env = os.Environ()
	}
	execCmd.Env = append(execCmd.Env, cmd.Env...)

	// TODO - would be nice to capture out/err and stream to debug log, to cut down on noise
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	log.Info().Msgf("Executing: %q", cmd.PrettyFormat())
	err := execCmd.Run()
	if err != nil {
		log.Error().Msgf("Command failed: %v", err)
		return &Error{Command: cmd, Err: err}
	}

	return nil
}

// Capture runs a Command, streaming stdout and stderr to the given writers.
// An error is returned if the command exits non-zero
func (r *RealRunner) Capture(cmd Command, stdout io.Writer, stderr io.Writer) error {
	panic("TODO")
}
