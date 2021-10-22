package shell

import (
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
