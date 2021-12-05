package shell

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

// Should definitely pass when we execute a single expected command
func TestMockRunnerPassesSingleCommand(t *testing.T) {
	m := DefaultMockRunner()
	m.Test(t)

	m.ExpectCmd(CmdFromArgs("FOO=BAR", "echo", "hello", "world"))

	err := m.Run(Command{
		Prog: "echo",
		Args: []string{"hello", "world"},
		Env:  []string{"FOO=BAR"},
	})

	assert.NoError(t, err)

	m.AssertExpectations(t)
}

// Should pass when we run multiple commands in order
func TestMockRunnerPassesMultipleCommandsInOrder(t *testing.T) {
	m := DefaultMockRunner()
	m.Test(t)

	m.ExpectCmd(CmdFromArgs("echo", "1"))
	m.ExpectCmd(CmdFromArgs("echo", "2"))

	var err error
	err = m.Run(CmdFromArgs("echo", "1"))
	assert.NoError(t, err)

	err = m.Run(CmdFromArgs("echo", "2"))
	assert.NoError(t, err)

	m.AssertExpectations(t)
}

// Should fail when commands are run out of order
//
// Note: This is tricky because we're trying to verify that MockRunner successfully triggers
// without, you know, actually failing this unit test. Since the MockRunner supports two failure
// modes (fail test or panic), we switch to panic and use recover() to verify the expected error occurred.
func TestMockRunnerFailsWhenOutOfOrder(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("mockRunner.Run() should have panicked, but it did not")
			return
		}

		matcher := regexp.MustCompile(`received out of order \(0 instead of 1\)`)
		assert.Regexp(t, matcher, r, "Unexpected panic message, did something else trigger a panic?")
	}()

	m := DefaultMockRunner()
	// DON'T pass in the test. We want to panic on failure we can detect whether the error happened with recover()
	// m.Test(t) // <- Don't do this

	m.ExpectCmd(CmdFromArgs("echo", "1"))
	m.ExpectCmd(CmdFromArgs("echo", "2"))

	_ = m.Run(CmdFromArgs("echo", "2")) // this will trigger a panic
	t.Errorf("This line of code should never be reached")
}

// If we aren't verifying order, out-of-order commands should be fine!
func TestMockRunnerOutOfOrderPassesWithNoVerify(t *testing.T) {
	m := NewMockRunner(MockOptions{VerifyOrder: false})
	m.Test(t)

	m.ExpectCmd(CmdFromArgs("echo", "1"))
	m.ExpectCmd(CmdFromArgs("echo", "2"))

	assert.Nil(t, m.Run(CmdFromArgs("echo", "2")))
	assert.Nil(t, m.Run(CmdFromArgs("echo", "1")))

	m.AssertExpectations(t)
}

// Verify our mock runner can be used to mock cases where Run() returns an error
func TestMockRunnerCanMockFailedCommands(t *testing.T) {
	m := DefaultMockRunner()
	m.Test(t)

	m.ExpectCmd(CmdFromArgs("fail", "1")).Exits(1)

	m.ExpectCmd(CmdFromArgs("fail", "2")).WithStderr("an\nerr\nmsg").Exits(2)

	var e error
	var exitErr *ExitError

	e = m.Run(CmdFromArgs("fail", "1"))
	assert.Error(t, e)
	assert.IsType(t, &ExitError{}, e)
	exitErr = e.(*ExitError)
	assert.Equal(t, 1, exitErr.ExitCode)
	assert.Equal(t, "", exitErr.Stderr)
	assert.Equal(t, `Command "fail 1" exited with status 1`, e.Error())

	e = m.Run(CmdFromArgs("fail", "2"))
	assert.Error(t, e)
	assert.IsType(t, &ExitError{}, e)
	exitErr = e.(*ExitError)
	assert.Equal(t, 2, exitErr.ExitCode)
	assert.Equal(t, "an\nerr\nmsg", exitErr.Stderr)
	assert.Equal(t, "Command \"fail 2\" exited with status 2:\nan\nerr\nmsg", e.Error())
}

// Verify our mock runner can be used to mock cases where Run() returns an error
func TestMockRunnerCanMockErrors(t *testing.T) {
	m := DefaultMockRunner()
	m.Test(t)

	m.ExpectCmd(CmdFromArgs("echo", "1")).Fails(fmt.Errorf("my error"))

	e := m.Run(CmdFromArgs("echo", "1"))
	assert.Error(t, e, "error should not be nil")
	assert.IsType(t, &Error{}, e)
	assert.Equal(t, `Command "echo 1" failed to start: my error`, e.Error())
}

// Verify our mock runner can be used to set expectations on mocks with shell.Command
func TestMockRunnerCanMockRawCmds(t *testing.T) {
	m := DefaultMockRunner()
	m.Test(t)

	m.ExpectCmd(Command{Prog: "echo", Args: []string{"1"}})

	e := m.Run(CmdFromArgs("echo", "1"))
	assert.Nil(t, e, "mock runner should not return an error")
}

// Verify our mock runner can ignore environment variables
func TestMockRunnerCanIgnoreSubsetOfEnvVars(t *testing.T) {
	m := NewMockRunner(MockOptions{
		IgnoreEnvVars: []string{"FOO"},
	})
	m.Test(t)

	m.ExpectCmd(Command{
		Prog: "ls",
		Env:  []string{"HOME=/home/jdoe", "FOO=BAR"},
	})

	e := m.Run(Command{
		Prog: "ls",
		Env:  []string{"HOME=/home/jdoe", "FOO=NOTBAR"},
	})

	assert.Nil(t, e, "mock runner should not return an error")
}

// Verify our mock runner can ignore dir attribute on commands
func TestMockRunnerCanIgnoreDir(t *testing.T) {
	m := NewMockRunner(MockOptions{
		IgnoreDir: true,
	})
	m.Test(t)

	m.ExpectCmd(Command{
		Prog: "ls",
		Dir:  "/tmp/foo",
	})

	e := m.Run(Command{
		Prog: "ls",
		Dir:  "/tmp/bar",
	})

	assert.Nil(t, e, "mock runner should not return an error")
}

// Check cmd dumps
func TestMockRunnerCanDumpCmdsDefault(t *testing.T) {
	m := NewMockRunner(MockOptions{DumpStyle: Default})

	m.ExpectCmd(CmdFromArgs("echo", "foo"))
	m.ExpectCmd(Command{Prog: "echo", Args: []string{"bar"}})

	w := bytes.NewBufferString("")
	e := m.dumpExpectedCmds(w)
	assert.Nil(t, e, "dumpExpectedCmds() should not return an error")
	expected := `

Expected commands:

	0 (0 matches):
	shell.Command{Prog:"echo", Args:[]string{"foo"}, Env:[]string(nil), Dir:"", PristineEnv:false}

	1 (0 matches):
	shell.Command{Prog:"echo", Args:[]string{"bar"}, Env:[]string(nil), Dir:"", PristineEnv:false}

`

	assert.Equal(t, expected, w.String())
}

// Check cmd dumps
func TestMockRunnerCanDumpCmdsPretty(t *testing.T) {
	m := NewMockRunner(MockOptions{DumpStyle: Pretty})

	m.ExpectCmd(CmdFromArgs("echo", "foo"))
	m.ExpectCmd(Command{Prog: "echo", Args: []string{"bar"}})

	w := bytes.NewBufferString("")
	e := m.dumpExpectedCmds(w)
	assert.Nil(t, e, "dumpExpectedCmds() should not return an error")

	expected := `

Expected commands:

	0 (0 matches): echo foo

	1 (0 matches): echo bar

`
	assert.Equal(t, expected, w.String())
}

// Check cmd dumps
func TestMockRunnerCanDumpCmdsSpew(t *testing.T) {
	m := NewMockRunner(MockOptions{DumpStyle: Spew})

	m.ExpectCmd(CmdFromArgs("echo", "foo"))
	m.ExpectCmd(Command{Prog: "echo", Args: []string{"bar"}})

	w := bytes.NewBufferString("")
	e := m.dumpExpectedCmds(w)
	assert.Nil(t, e, "dumpExpectedCmds() should not return an error")
	expected := `

Expected commands:

	0 (0 matches): echo foo

(shell.Command) {
	Prog: (string) (len=4) "echo",
	Args: ([]string) (len=1) {
		(string) (len=3) "foo"
	},
	Env: ([]string) <nil>,
	Dir: (string) "",
	PristineEnv: (bool) false
}

	1 (0 matches): echo bar

(shell.Command) {
	Prog: (string) (len=4) "echo",
	Args: ([]string) (len=1) {
		(string) (len=3) "bar"
	},
	Env: ([]string) <nil>,
	Dir: (string) "",
	PristineEnv: (bool) false
}

`
	assert.Equal(t, expected, w.String())
}

func TestCmdFromFmt(t *testing.T) {
	expected := Command{
		Prog: "ls",
		Args: []string{"-al", "/var"},
		Env:  []string{"FOO=BAR", "HOME=/tmp"},
	}

	actual := CmdFromFmt("FOO=%s HOME=%s ls -al %s", "BAR", "/tmp", "/var")
	assert.Equal(t, expected, actual)
}
