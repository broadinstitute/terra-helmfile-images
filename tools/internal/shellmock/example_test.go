package shellmock

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	. "github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock/matchers"
	"regexp"
	"testing"
)

// Example tests demonstrating how to use the shellmock package

// The code we're testing:
func SayHello(runner shell.Runner) {
	// let's run a bunch of pointless commands!
	// note: errors ignored for clarity
	_ = runner.RunWithArgs("echo", "hello", "friend")
	_ = runner.RunWithArgs("echo", "hello", "bud")
	_ = runner.RunWithArgs("echo", "hello", "stranger")

	// environment variables!!! whoah
	_ = runner.Run(shell.Command{
		Prog: "echo",
		Args: []string{"hello", "$TITLE"},
		Env:  []string{"TITLE=pal"},
	})

	_ = runner.Run(shell.Command{
		Prog: "ls",
		Args: []string{"~/.ssh"},
		Env:  []string{"HOME=/root"},
	})

	_ = runner.RunWithArgs("echo", "hiiiiii")

	_ = runner.Run(shell.Command{
		Prog: "echo",
		Env:  []string{"FOO=doesntmatter"},
	})

	_ = runner.Run(shell.Command{
		Prog: "ls",
		Dir: "/tmp",
	})

}

// The test:
func TestHello(t *testing.T) {
	runner := DefaultMockRunner()

	// Recommended: Pass test object to mock runner so that:
	//  * expected/actual call mismatches will trigger a test failure instead of a panic
	//  * additional debugging output will be dumped on test failure
	runner.Test(t)

	// use OnCmd() to tell the mock that we expect a specific command to be run
	runner.OnCmd(CmdWithArgs("echo", "hello", "friend"))
	runner.OnCmd(CmdWithArgs("echo", "hello", "bud"))
	runner.OnCmd(CmdWithArgs("echo", "hello", "stranger"))
	runner.OnCmd(CmdWithArgs("echo", "hello", "$TITLE").WithEnvVar("TITLE", "pal"))
	runner.OnCmd(CmdWithArgs("ls", "~/.ssh").WithExactEnvVars("HOME=/root"))

	// MatchesRegexp() can be used to check that an attribute matches a regular expression.
	// Eg. This requires the command to have an argument at index 0 that starts with "h"
	runner.OnCmd(CmdWithArgs("echo").
		WithArg(MatchesRegexp(regexp.MustCompile("^h"))),
	)

	// AnyString() can be used to match any string.
	// Eg. This requires an env var to exist, but we don't care what the value is
	runner.OnCmd(CmdWithArgs("echo").
		WithEnvVar("FOO", AnyString()))

	// Directories can be matched using the same matchers as other attributes
	runner.OnCmd(CmdWithArgs("ls").WithDir("/tmp"))

	// ok ok, let's test the code already
	SayHello(runner)

	// !!! IMPORTANT !!!
	// make sure to call AssertExpectations on the testify mock to verify all the
	// expected commands were actually run.
	runner.AssertExpectations(t)
}
