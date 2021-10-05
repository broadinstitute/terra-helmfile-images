package shellmock

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

// Should definitely pass when we execute a single expected command
func TestMockRunnerPassesSingleCommand(t *testing.T) {
	m := DefaultMockRunner()
	m.ExpectCmdS(t, "FOO=BAR echo hello world")

	assert.Nil(t, m.Run(CmdFromFmt("FOO=BAR echo hello world")))

	m.mock.AssertExpectations(t)
	m.mock.AssertNumberOfCalls(t, "Run", 1)
}

// Should pass when we run multiple commands in order
func TestMockRunnerPassesMultipleCommandsInOrder(t *testing.T) {
	m := DefaultMockRunner()
	m.ExpectCmdS(t, "echo 1")
	m.ExpectCmdS(t, "echo 2")

	assert.Nil(t, m.Run(CmdFromFmt("echo 1")))
	assert.Nil(t, m.Run(CmdFromFmt("echo 2")))

	m.mock.AssertExpectations(t)
	m.mock.AssertNumberOfCalls(t, "Run", 2)
}

// Should fail when commands are run out of order
//
// Note: This is tricky because we're trying to verify that MockRunner successfully triggers
// without, you know, actually failing this unit test. Since the MockRunner supports two failure
// modes (fail test or panic), we switch to panic and use recover() to verify an error occurred.
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

	// Use panic mode so we can detect whether it happened it with recover()
	m := NewMockRunner(Options{VerifyOrder: true, FailureMode: Panic})
	m.ExpectCmdS(t, "echo 1")
	m.ExpectCmdS(t, "echo 2")

	_ = m.Run(CmdFromFmt("echo 2")) // this will trigger a panic
	t.Errorf("This line of code should never be reached")
}

// If we aren't verifying order, out-of-order commands should be fine!
func TestMockRunnerOutOfOrderPassesWithNoVerify(t *testing.T) {
	m := NewMockRunner(Options{VerifyOrder: false})
	m.ExpectCmdS(t, "echo 1")
	m.ExpectCmdS(t, "echo 2")

	assert.Nil(t, m.Run(CmdFromFmt("echo 2")))
	assert.Nil(t, m.Run(CmdFromFmt("echo 1")))

	m.mock.AssertExpectations(t)
	m.mock.AssertNumberOfCalls(t, "Run", 2)
}

// Verify our mock runner can be used to mock cases where Run() returns an error
func TestMockRunnerCanMockErrors(t *testing.T) {
	m := DefaultMockRunner()
	m.ExpectCmdFmt(t, "echo 1").Return(fmt.Errorf("my error"))

	e := m.Run(CmdFromFmt("echo 1"))
	assert.Error(t, e, "error should not be nil")
	assert.Errorf(t, e, "my error", "mock runner should return the mocked error")
}

func TestCmdFmt(t *testing.T) {
	testCases := []struct {
		description string
		format      string
		args        []string
		expected    shell.Command
	}{
		{
			description: "Empty",
			format:      "",
			expected:    shell.Command{},
		},
		{
			description: "No args, no substitution",
			format:      "echo",
			expected: shell.Command{
				Prog: "echo",
			},
		},
		{
			description: "With one arg, no substitution",
			format:      "echo hello",
			expected: shell.Command{
				Prog: "echo",
				Args: []string{"hello"},
			},
		},
		{
			description: "With two args and substitution",
			format:      "echo hello world",
			expected: shell.Command{
				Prog: "echo",
				Args: []string{"hello", "world"},
			},
		},
		{
			description: "Multiple env vars with substitutions",
			format:      "FOO=%s EMPTY= HOME=%s _n=data LANG=en echo %s %s",
			args:        []string{"BAR", "/root", "hello", "world"},
			expected: shell.Command{
				Prog: "echo",
				Args: []string{"hello", "world"},
				Env:  []string{"FOO=BAR", "EMPTY=", "HOME=/root", "_n=data", "LANG=en"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			// convert []string to []interface...
			argsi := make([]interface{}, len(testCase.args))
			for i, s := range testCase.args {
				argsi[i] = s
			}
			actual := CmdFromFmt(testCase.format, argsi...)
			assert.Equal(t, testCase.expected, actual)
		})
	}
}
