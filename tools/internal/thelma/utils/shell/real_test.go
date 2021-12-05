package shell

import (
	"bytes"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	tmpdir := t.TempDir()

	runner := NewDefaultRunner()
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
	runner := NewDefaultRunner()
	cmd := Command{}
	cmd.Prog = "sh"
	cmd.Args = []string{"-c", "echo oops >&2 && exit 42"}
	cmd.Dir = ""

	err := runner.Run(cmd)
	if err == nil {
		t.Errorf("Expected error when running command: %v", cmd)
	}
	exitErr, ok := err.(*ExitError)
	if !assert.True(t, ok, "Expected shell.ExitError, got: %v", err) {
		t.FailNow()
	}
	assert.Equal(t, "Command \"sh -c echo oops >&2 && exit 42\" exited with status 42:\noops\n", exitErr.Error())
	assert.Equal(t, "oops\n", string(exitErr.Stderr))
	assert.Equal(t, 42, exitErr.ExitCode)
}

func TestRunError(t *testing.T) {
	runner := NewDefaultRunner()
	cmd := Command{}
	cmd.Prog = "echo"
	cmd.Args = []string{"a", "b"}
	cmd.Dir = path.Join(t.TempDir(), "this-file-does-not-exist")

	err := runner.Run(cmd)
	if err == nil {
		t.Errorf("Expected error when running command: %v", cmd)
	}
	_err, ok := err.(*Error)
	if !assert.True(t, ok, "Expected shell.Error, got: %v", err) {
		t.FailNow()
	}
	assert.Regexp(t, "Command \"echo a b\" failed to start", _err.Error())
}

func TestRunWith(t *testing.T) {
	runner := NewDefaultRunner()
	var err error

	stdout := bytes.NewBuffer([]byte{})
	err = runner.RunWith(
		Command{
			Prog: "echo",
			Args: []string{"hello"},
		},
		RunOptions{
			Stdout: stdout,
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, "hello\n", stdout.String())

	stderr := bytes.NewBuffer([]byte{})
	err = runner.RunWith(
		Command{
			Prog: "ls",
			Args: []string{path.Join(t.TempDir(), "does-not-exist")},
		},
		RunOptions{
			Stderr: stderr,
		},
	)
	assert.Error(t, err)
	assert.Regexp(t, "does-not-exist.*No such file or directory", stderr.String())
}

func TestCapturingWriterRollover(t *testing.T) {
	var n int
	var err error

	writer := newCapturingWriter(4, log.Logger, nil)
	assert.Equal(t, 4, writer.maxLen)

	// writing a message shorter than maxLen should trigger a rollover
	n, err = writer.Write([]byte("abcd"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, 4, writer.len)
	assert.Equal(t, "abcd", writer.String())

	// writing a message longer than maxLen should trigger a rollover
	n, err = writer.Write([]byte("egfhi"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, 0, writer.len)
	assert.Equal(t, "", writer.String())

	// buffer should not include any previously written data
	n, err = writer.Write([]byte("jkl"))
	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, 3, writer.len)
	assert.Equal(t, "jkl", writer.String())

	// one more rollover for funsies
	n, err = writer.Write([]byte("mn"))
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, 2, writer.len)
	assert.Equal(t, "mn", writer.String())
}
