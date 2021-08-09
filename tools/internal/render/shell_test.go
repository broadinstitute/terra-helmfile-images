package render

import (
	"os"
	"path"
	"regexp"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	// Create tmp dir and remove it
	tmpdir, err := os.MkdirTemp("", "shell-test")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(tmpdir)

	runner := RealRunner{}
	cmd := Command{}
	cmd.Prog = "mkdir"
	cmd.Args = []string{ "test-dir-1" }
	cmd.Dir = tmpdir

	err = runner.Run(cmd)
	if err != nil {
		t.Error(err)
	}

	// Verify that the command was run and created the directory
	testDir := path.Join(tmpdir, "test-dir-1")
	f, err := os.Stat(testDir)
	if err != nil {
		t.Errorf("testDir does not exist: %v", err)
	}
	if !f.IsDir() {
		t.Errorf("testDir is not directory: %v", f)
	}
}

func TestRunFailed(t *testing.T) {
	runner := RealRunner{}
	cmd := Command{}
	cmd.Prog = "sh"
	cmd.Args = []string{ "-c", "exit 42" }
	cmd.Dir = ""

	err := runner.Run(cmd)
	if err == nil {
		t.Errorf("Expected error when running command: %v", cmd)
	}
	shellErr, ok := err.(*ShellError)
	if !ok {
		t.Errorf("Expected ShellError, got: %v", err)
	}
	if !regexp.MustCompile("Command \"sh -c exit 42\" exited with status 42").MatchString(shellErr.Error()) {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestRunError(t *testing.T) {
	runner := RealRunner{}
	cmd := Command{}
	cmd.Prog = "echo"
	cmd.Args = []string{ "a", "b" }
	cmd.Dir = "/this-file-does-not-exist-398u48"

	err := runner.Run(cmd)
	if err == nil {
		t.Errorf("Expected error when running command: %v", cmd)
	}
	shellErr, ok := err.(*ShellError)
	if !ok {
		t.Errorf("Expected ShellError, got: %v", err)
	}
	if !regexp.MustCompile("Command \"echo a b\" failed to start").MatchString(shellErr.Error()) {
		t.Errorf("Unexpected error message: %v", err)
	}
}

