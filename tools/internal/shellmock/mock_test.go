package shellmock

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock/matchers"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

// Should definitely pass when we execute a single expected command
func TestMockRunnerPassesSingleCommand(t *testing.T) {
	m := DefaultMockRunner()
	m.Test(t)

	m.OnCmd(matchers.CmdWithEnv("FOO=BAR", "echo", "hello", "world"))

	assert.Nil(t, m.Run(shell.Command{
		Prog: "echo",
		Args: []string{"hello", "world"},
		Env: []string{"FOO=BAR"},
	}))

	m.AssertExpectations(t)
	m.AssertNumberOfCalls(t, "Run", 1)
}

// Should pass when we run multiple commands in order
func TestMockRunnerPassesMultipleCommandsInOrder(t *testing.T) {
	m := DefaultMockRunner()
	m.Test(t)

	m.OnCmd(matchers.CmdWithArgs("echo", "1"))
	m.OnCmd(matchers.CmdWithArgs("echo", "2"))

	assert.Nil(t, m.RunWithArgs("echo", "1"))
	assert.Nil(t, m.RunWithArgs("echo", "2"))

	m.AssertExpectations(t)
	m.AssertNumberOfCalls(t, "Run", 2)
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

	m.OnCmd(matchers.CmdWithArgs("echo", "1"))
	m.OnCmd(matchers.CmdWithArgs("echo", "2"))

	_ = m.RunWithArgs("echo", "2") // this will trigger a panic
	t.Errorf("This line of code should never be reached")
}

// If we aren't verifying order, out-of-order commands should be fine!
func TestMockRunnerOutOfOrderPassesWithNoVerify(t *testing.T) {
	m := NewMockRunner(Options{VerifyOrder: false})
	m.Test(t)

	m.OnCmd(matchers.CmdWithArgs("echo", "1"))
	m.OnCmd(matchers.CmdWithArgs("echo", "2"))

	assert.Nil(t, m.RunWithArgs("echo", "2"))
	assert.Nil(t, m.RunWithArgs("echo", "1"))

	m.AssertExpectations(t)
	m.AssertNumberOfCalls(t, "Run", 2)
}

// Verify our mock runner can be used to mock cases where Run() returns an error
func TestMockRunnerCanMockErrors(t *testing.T) {
	m := DefaultMockRunner()
	m.Test(t)

	m.OnCmd(matchers.CmdWithArgs("echo", "1")).Return(fmt.Errorf("my error"))

	e := m.RunWithArgs("echo", "1")
	assert.Error(t, e, "error should not be nil")
	assert.Errorf(t, e, "my error", "mock runner should return the mocked error")
}
