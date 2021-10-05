package shellmock

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"testing"
)

// Example test suite demonstrating how to use the shellmock package

// The code we're testing:
func SayHello(runner shell.Runner) {
	// let's run a bunch of pointless commands!
	// note: errors ignored for simplicity
	_ = runner.Run(shell.Command{
		Prog: "echo",
		Args: []string{"hello", "friend"},
	})

	_ = runner.Run(shell.Command{
		Prog: "echo",
		Args: []string{"hello", "bud"},
	})

	_ = runner.Run(shell.Command{
		Prog: "echo",
		Args: []string{"hello", "stranger"},
	})

	// environment variables!!! whoah
	_ = runner.Run(shell.Command{
		Prog: "echo",
		Args: []string{"hello", "$TITLE"},
		Env:  []string{"TITLE=pal"},
	})
}

// The test:
func TestHello(t *testing.T) {
	runner := DefaultMockRunner() // that's shellmock.DefaultMockRunner() to you, bub

	// use ExpectCmd() to tell the mock that we expect a specific command to be run
	runner.ExpectCmd(t, shell.Command{
		Prog: "echo",
		Args: []string{"hello", "friend"},
	})

	// ExpectCmdS and ExpectCmdFmt are convenience wrappers around ExpectCmd,
	// so we can avoid instantiating a big struct each time.
	runner.ExpectCmdS(t, "echo hello bud")
	runner.ExpectCmdFmt(t, "echo hello %s", "stranger") // useful if you have to inject something dynamic

	// both ExpectCmdS and ExpectCmdFmt automatically parse env variables at the beginning of a command
	runner.ExpectCmdS(t, "TITLE=pal echo hello $TITLE")

	// ok ok, let's test the code already
	SayHello(runner)

	// !!! IMPORTANT !!!
	// make sure to call AssertExpectations on the testify mock to verify all the
	// expected commands were actually "run".
	runner.AssertExpectations(t)
}
