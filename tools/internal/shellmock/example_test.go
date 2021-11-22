package shellmock

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"testing"
)

// Example tests demonstrating how to use the shellmock package

// The code we're testing:
func SayHello(runner shell.Runner) error {
	// let's run a bunch of pointless commands!
	// note: errors ignored for clarity
	return runner.Run(CmdFromArgs("echo", "hello", "world"))
}

// The test:
func TestHello(t *testing.T) {
	runner := DefaultMockRunner()

	// Recommended: Pass test object to mock runner so that:
	//  * expected/actual call mismatches will trigger a test failure instead of a panic
	//  * additional debugging output will be dumped on test failure
	runner.Test(t)

	// use ExpectCmd() to tell the mock that we expect a specific command to be run
	runner.ExpectCmd(CmdFromArgs("echo", "hello", "world"))

	// test the code
	err := SayHello(runner)
	assert.NoError(t, err)

	// !!! IMPORTANT !!!
	// make sure to call AssertExpectations on the testify mock to verify all the
	// expected commands were actually run.
	runner.AssertExpectations(t)
}
