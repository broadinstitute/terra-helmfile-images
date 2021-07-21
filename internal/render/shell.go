package render

import (
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
)

/*
Wrapper around exec.Command to support interface-based mocking for unit testing

https://joshrendek.com/2014/06/go-lang-mocking-exec-dot-command-using-interfaces/
*/
type ShellRunner interface {
	Run(cmd Command) error
}

type Command struct {
	Prog string
	Args []string
	Dir string
}

type RealRunner struct {}

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
		return err
	}

	return nil
}