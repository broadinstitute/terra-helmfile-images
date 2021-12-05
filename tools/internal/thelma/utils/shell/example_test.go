package shell

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

// Example tests demonstrating how to use the shellmock package

// The code we're testing:

// SayHello simply echos hello world
func SayHello(runner Runner) error {
	return runner.Run(Command{
		Prog: "echo",
		Args: []string{"hello", "world"},
	})
}

// ListTmpFiles returns a list of files in the /tmp directory
func ListTmpFiles(runner Runner) ([]string, error) {
	cmd := Command{
		Prog: "ls",
		Args: []string{"-1", "/tmp"},
	}

	buf := bytes.NewBuffer([]byte{})

	err := runner.RunWith(cmd, RunOptions{Stdout: buf})
	if err != nil {
		return nil, err
	}

	stdout := buf.String()
	stdout = strings.TrimSuffix(stdout, "\n")
	return strings.Split(stdout, "\n"), nil
}

func ExitStatus42IsFine(runner Runner) error {
	err := runner.Run(Command{
		Prog: "flaky-cmd",
	})
	if err == nil {
		return nil
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		return err // not an exit error
	}
	if exitErr.ExitCode == 42 {
		log.Warn().Msgf("flaky-cmd exited 42, but that's ok")
		return nil
	}
	return exitErr
}

// The tests:
func TestHello(t *testing.T) {
	runner := DefaultMockRunner()

	// Recommended: Pass test object to mock runner so that:
	//  * expected/actual call mismatches will trigger a test failure instead of a panic
	//  * additional debugging output will be dumped on test failure
	runner.Test(t)

	// use ExpectCmd() to tell the mock that we expect a specific command to be run
	runner.ExpectCmd(Command{
		Prog: "echo",
		Args: []string{"hello", "world"},
	})

	// test the code
	err := SayHello(runner)
	assert.NoError(t, err)

	// !!! IMPORTANT !!!
	// make sure to call AssertExpectations on the testify mock to verify all the
	// expected commands were actually run.
	runner.AssertExpectations(t)
}

func TestListTmpFiles(t *testing.T) {
	runner := DefaultMockRunner()
	runner.Test(t)

	// CmdFromArgs is convenience function quickly generating Command structs
	// CmdFromFmt provides similar functionality, but using format string + args
	cmd := CmdFromArgs("ls", "-1", "/tmp")

	runner.ExpectCmd(cmd).WithStdout("hello.txt\nzzzz.data\n")

	// verify output was parsed correctly
	files, err := ListTmpFiles(runner)
	assert.NoError(t, err)
	assert.Equal(t, []string{"hello.txt", "zzzz.data"}, files)

	runner.AssertExpectations(t)
}

func TestExitStatus42IsFine(t *testing.T) {
	runner := DefaultMockRunner()
	runner.Test(t)

	// CmdFromArgs is convenience function quickly generating Command structs
	// CmdFromFmt provides similar functionality, but using format string + args
	runner.ExpectCmd(CmdFromArgs("flaky-cmd")).Exits(42)
	runner.ExpectCmd(CmdFromArgs("flaky-cmd")).Exits(43)

	var err error

	// verify the first run returns no error
	err = ExitStatus42IsFine(runner)
	assert.NoError(t, err)

	// verify the first run DOES return an error
	err = ExitStatus42IsFine(runner)
	assert.Error(t, err)
	assert.IsType(t, &ExitError{}, err)

	runner.AssertExpectations(t)
}
