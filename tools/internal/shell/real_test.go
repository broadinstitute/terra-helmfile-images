package shell

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"regexp"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	tmpdir := t.TempDir()

	runner := NewRealRunner()
	cmd := Command{}
	cmd.Prog = "sh"
	cmd.Env = []string{"VAR1=foo"}
	cmd.Args = []string{"-c", "mkdir test-dir-$VAR1"}
	cmd.Dir = tmpdir

	if err := runner.Run(cmd); err != nil {
		t.Error(err)
	}

	// Verify that the command was run and created the directory
	testDir := path.Join(tmpdir, "test-dir-foo")
	f, err := os.Stat(testDir)
	if err != nil {
		t.Errorf("testDir does not exist: %v", err)
	}
	if !f.IsDir() {
		t.Errorf("testDir is not directory: %v", f)
	}
}

func TestRunFailed(t *testing.T) {
	runner := NewRealRunner()
	cmd := Command{}
	cmd.Prog = "sh"
	cmd.Args = []string{"-c", "exit 42"}
	cmd.Dir = ""

	err := runner.Run(cmd)
	if err == nil {
		t.Errorf("Expected error when running command: %v", cmd)
	}
	shellErr, ok := err.(*Error)
	if !ok {
		t.Errorf("Expected ShellError, got: %v", err)
	}
	if !regexp.MustCompile("Command \"sh -c exit 42\" exited with status 42").MatchString(shellErr.Error()) {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestRunError(t *testing.T) {
	runner := NewRealRunner()
	cmd := Command{}
	cmd.Prog = "echo"
	cmd.Args = []string{"a", "b"}
	cmd.Dir = "/this-file-does-not-exist-398u48"

	err := runner.Run(cmd)
	if err == nil {
		t.Errorf("Expected error when running command: %v", cmd)
	}
	shellErr, ok := err.(*Error)
	if !ok {
		t.Errorf("Expected ShellError, got: %v", err)
	}
	if !regexp.MustCompile("Command \"echo a b\" failed to start").MatchString(shellErr.Error()) {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestCmdFromTokens(t *testing.T) {
	testCases := []struct {
		description string
		tokens      []string
		expected    Command
	}{
		{
			description: "Empty",
			tokens:      []string{},
			expected:    Command{},
		},
		{
			description: "No args",
			tokens:      []string{"echo"},
			expected: Command{
				Prog: "echo",
			},
		},
		{
			description: "With one arg",
			tokens:      []string{"echo", "hello"},
			expected: Command{
				Prog: "echo",
				Args: []string{"hello"},
			},
		},
		{
			description: "With two args",
			tokens:      []string{"echo", "hello", "world"},
			expected: Command{
				Prog: "echo",
				Args: []string{"hello", "world"},
			},
		},
		{
			description: "Many args",
			tokens: []string{"echo", "the", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog"},
			expected: Command{
				Prog: "echo",
				Args: []string{"the", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog"},
			},
		},
		{
			description: "Single env var",
			tokens: []string{"FOO=BAR", "echo", "hello", "world"},
			expected: Command{
				Prog: "echo",
				Args: []string{"hello", "world"},
				Env:  []string{"FOO=BAR"},
			},
		},
		{
			description: "Multiple env vars",
			tokens: []string{"FOO=BAR", "EMPTY=", "HOME=/root", "_n=data", "LANG=en", "echo", "hello", "world"},
			expected: Command{
				Prog: "echo",
				Args: []string{"hello", "world"},
				Env:  []string{"FOO=BAR", "EMPTY=", "HOME=/root", "_n=data", "LANG=en"},
			},
		},
		{
			description: "Only env vars",
			tokens: []string{"FOO=BAR", "EMPTY=", "HOME=/root", "_n=data", "LANG=en"},
			expected: Command{
				Prog: "",
				Env:  []string{"FOO=BAR", "EMPTY=", "HOME=/root", "_n=data", "LANG=en"},
			},
		},
		{
			description: "Env vars no args",
			tokens: []string{"FOO=BAR", "EMPTY=", "HOME=/root", "_n=data", "LANG=en", "echo"},
			expected: Command{
				Prog: "echo",
				Env:  []string{"FOO=BAR", "EMPTY=", "HOME=/root", "_n=data", "LANG=en"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			actual := CmdFromTokens(testCase.tokens...)
			assert.Equal(t, testCase.expected, actual)
		})
	}
}
