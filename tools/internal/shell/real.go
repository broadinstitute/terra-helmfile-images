package shell

import (
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
	"strings"
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

// RunS is a convenience wrapper around Run.
// Given a list of string arguments, RunS calls
// CmdFromTokens() to create a command passes it to Run()
//
// Eg. RunS("FOO=BAR", "HOME=/tmp", "ls", "-al", "~")
// will create a new Command{
//   Env: []string{"FOO=BAR", "HOME=/tmp"},
//   Prog: "ls",
//   Args: []string{"-al", "~"}
// }
// and pass it to Run()
//
func (r *RealRunner) RunS(args ...string) error {
	return r.Run(CmdFromTokens(args...))
}

// CmdFromTokens converts list of string arguments to a Command.
// Eg. CmdFromTokens("FOO=BAR", "HOME=/tmp", "ls", "-al", "~")
// ->
// Command{
//   Env: []string{"FOO=BAR", "HOME=/tmp"},
//   Prog: "ls",
//   Args: []string{"-al", "~"},
// }
func CmdFromTokens(args ...string) Command {
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

	cmd := Command{}

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
