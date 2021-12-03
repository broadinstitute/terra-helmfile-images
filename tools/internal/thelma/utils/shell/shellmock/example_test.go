package shellmock

import (
	"bytes"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

// Example tests demonstrating how to use the shellmock package

// The code we're testing:

// SayHello simply echos hello world
func SayHello(runner shell.Runner) error {
	return runner.Run(shell.Command{
		Prog: "echo",
		Args: []string{"hello", "world"},
	})
}

// ListTmpFiles returns a list of files in the /tmp directory
func ListTmpFiles(runner shell.Runner) ([]string, error) {
	cmd := shell.Command{
		Prog: "ls",
		Args: []string{"-1", "/tmp"},
	}

	buf := bytes.NewBuffer([]byte{})

	err := runner.Capture(cmd, buf, nil)
	if err != nil {
		return nil, err
	}

	stdout := buf.String()
	stdout = strings.TrimSuffix(stdout, "\n")
	return strings.Split(stdout, "\n"), nil
}

// The tests:
func TestHello(t *testing.T) {
	runner := DefaultMockRunner()

	// Recommended: Pass test object to mock runner so that:
	//  * expected/actual call mismatches will trigger a test failure instead of a panic
	//  * additional debugging output will be dumped on test failure
	runner.Test(t)

	// use ExpectCmd() to tell the mock that we expect a specific command to be run
	runner.ExpectCmd(shell.Command{
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

func TestCaptureOutput(t *testing.T) {
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
