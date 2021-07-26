package render

import (
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

/* This file contains an integration test for the render utility */
var fakeEnvironments = []Environment{
	{name: "dev", base: "live"},
	{name: "alpha", base: "live"},
	{name: "jdoe", base: "personal"},
}

/* Struct for tracking global state that is mocked when a test executes and restored/cleaned up after */
type TestState struct {
	originalRunner         *ShellRunner
	mockRunner             *MockRunner
	originalConfigRepoPath string
	mockConfigRepoPath     string
	mockChartDir           string
	tmpDir                 string
}

/* Used in MockRunner to verify expected commands have been called */
type ExpectedCommand struct {
	Command   Command // Expected Command
	MockError error   // Optional error to return (for faking an error running the command)
}

/* MockRunner stores an ordered list (slice) of expected commands */
type MockRunner struct {
	expectedCommands []ExpectedCommand // Ordered list of expected commands
}

/*
Mock implementation of ShellCommand#Run

Instead of executing the command, compare it to the runner's list of expected commands, throwing an error on mismatch
*/
func (m *MockRunner) Run(cmd Command) error {
	if len(m.expectedCommands) == 0 {
		return fmt.Errorf("MockRunner: Received unexpected command %v", cmd)
	}

	var expected ExpectedCommand
	expected, m.expectedCommands = m.expectedCommands[0], m.expectedCommands[1:]

	if diff := cmp.Diff(cmd, expected.Command); diff != "" {
		return fmt.Errorf("MockRunner: %T differ (-got, +want): %s", expected.Command, diff)
	}

	return expected.MockError
}

/*
A table-driven integration test for the render tool.

Given a list of CLI arguments to the render command, the test verifies
that the correct underlying `helmfile` command(s) are run.

Reference:
https://gianarb.it/blog/golang-mockmania-cli-command-with-cobra
*/
func TestRender(t *testing.T) {
	// Set up mocked global state before tests run
	ts, err := setup()
	if err != nil {
		t.Error(err)
		return
	}

	// Add cleanup hook to restore global state after tests complete
	t.Cleanup(func() {
		err := cleanup(ts)
		if err != nil {
			t.Error(err)
		}
	})

	var tests = []struct {
		description      string            // Testcase description
		arguments        []string          // Fake user-supplied CLI arguments to pass to `render`
		expectedCommands []ExpectedCommand // Ordered list of CLI commands we expect `render` to run
		expectedError    *regexp.Regexp    // Optional error we expect to be raised
	}{
		{
			description:   "invalid argument",
			arguments:     args("--foo"),
			expectedError: regexp.MustCompile("unknown flag"),
		},
		{
			description:   "-a should require -e",
			arguments:     args("-a foo"),
			expectedError: regexp.MustCompile("environment must be specified"),
		},
		{
			description:   "--app-version should require -a",
			arguments:     args("--app-version 1.0.0"),
			expectedError: regexp.MustCompile("--app-version requires an app be specified with -a"),
		},
		{
			description:   "--chart-version should require -a",
			arguments:     args("--chart-version 1.0.0"),
			expectedError: regexp.MustCompile("--chart-version requires an app be specified with -a"),
		},
		{
			description:   "--chart-dir should require -a",
			arguments:     args("--chart-dir %s", ts.mockChartDir),
			expectedError: regexp.MustCompile("--chart-dir requires an app be specified with -a"),
		},
		{
			description:   "--chart-dir and --chart-version incompatible",
			arguments:     args("-e dev -a leonardo --chart-dir %s --chart-version 1.0.0", ts.mockChartDir),
			expectedError: regexp.MustCompile("only one of --chart-dir or --chart-version may be specified"),
		},
		{
			description:   "--chart-dir must exist",
			arguments:     args("-e dev -a leonardo --chart-dir path/to/nowhere"),
			expectedError: regexp.MustCompile("chart directory does not exist: path/to/nowhere"),
		},
		{
			description:   "--argocd and --app-version incompatible",
			arguments:     args("-e dev -a leonardo --app-version 1.0.0 --argocd"),
			expectedError: regexp.MustCompile("--argocd cannot be used with --chart-dir, --chart-version, or --app-version"),
		},
		{
			description:   "--argocd and --chart-version incompatible",
			arguments:     args("-e dev -a leonardo --chart-version 1.0.0 --argocd"),
			expectedError: regexp.MustCompile("--argocd cannot be used with --chart-dir, --chart-version, or --app-version"),
		},
		{
			description:   "--argocd and --chart-dir incompatible",
			arguments:     args("-e dev -a leonardo --chart-dir=%s --argocd", ts.mockChartDir),
			expectedError: regexp.MustCompile("--argocd cannot be used with --chart-dir, --chart-version, or --app-version"),
		},
		{
			description: "incorrect environment should return error",
			arguments:   args("-e foo"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
			},
			expectedError: regexp.MustCompile("unknown environment: foo"),
		},
		{
			description: "no arguments should render for all environments",
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e alpha --selector=group=terra template --skip-deps --output-dir=%s/output/alpha", ts.mockConfigRepoPath),
				ts.cmd("helmfile --log-level=info -e dev   --selector=group=terra template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
				ts.cmd("helmfile --log-level=info -e jdoe  --selector=group=terra template --skip-deps --output-dir=%s/output/jdoe", ts.mockConfigRepoPath),
			},
		},
		{
			description: "--argocd without -e or -a should render Argo manifests for all environments",
			arguments:   args("--argocd"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e alpha --selector=group=argocd template --skip-deps --output-dir=%s/output/alpha", ts.mockConfigRepoPath),
				ts.cmd("helmfile --log-level=info -e dev   --selector=group=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
				ts.cmd("helmfile --log-level=info -e jdoe  --selector=group=argocd template --skip-deps --output-dir=%s/output/jdoe", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-e should render for specific environment",
			arguments:   args("-e dev"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e dev --selector=group=terra template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-e with --argocd should render ArgoCD manifests for specific environment",
			arguments:   args("-e dev --argocd"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e dev --selector=group=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-a should render for specific service",
			arguments:   args("-e dev -a leonardo"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e dev --selector=app=leonardo,group=terra template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-a with --argocd should render ArgoCD manifests for specific service",
			arguments:   args("--argocd -e dev -a leonardo"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e dev --selector=app=leonardo,group=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-a with --app-version should set app version",
			arguments:   args("-e dev -a leonardo --app-version 1.2.3"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e dev --selector=app=leonardo,group=terra --state-values-set=releases.leonardo.appVersion=1.2.3 template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-a with --chart-dir should set chart dir and not include --skip-deps",
			arguments:   args("-e dev -a leonardo --chart-dir=%s", ts.mockChartDir),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e dev --selector=app=leonardo,group=terra --state-values-set=releases.leonardo.repo=%s template --output-dir=%s/output/dev", ts.mockChartDir, ts.mockConfigRepoPath),
			},
		},
		{
			description: "-a with --app-version and --chart-version should set both",
			arguments:   args(" -e dev -a leonardo --app-version 1.2.3 --chart-version 4.5.6"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e dev --selector=app=leonardo,group=terra --state-values-set=releases.leonardo.appVersion=1.2.3,releases.leonardo.chartVersion=4.5.6 template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "should fail if repo update fails",
			expectedCommands: []ExpectedCommand{
				ts.failCmd("dieee", "helmfile --log-level=info --allow-no-matching-release repos"),
			},
			expectedError: regexp.MustCompile("dieee"),
		},
		{
			description: "should fail if helmfile template fails",
			arguments:   args("-e dev"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.failCmd("dieee", "helmfile --log-level=info -e dev --selector=group=terra template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
			expectedError: regexp.MustCompile("dieee"),
		},
		{
			description: "should run helmfile with --log-level=debug if run with -v -v",
			arguments:   args("-e dev -v -v"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=debug --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=debug -e dev --selector=group=terra template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "--stdout should not render to output directory",
			arguments:   args("--env=alpha --stdout"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e alpha --selector=group=terra template --skip-deps"),
			},
		},
		{
			description: "-d should render to custom output directory",
			arguments:   args("-e jdoe -d path/to/nowhere"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("helmfile --log-level=info -e jdoe --selector=group=terra template --skip-deps --output-dir=path/to/nowhere/jdoe"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			mockRunner := ts.mockRunner
			mockRunner.expectedCommands = test.expectedCommands

			err := ExecuteWithCallback(func(cobraCmd *cobra.Command) {
				cobraCmd.SetArgs(test.arguments)
			})

			if err == nil && test.expectedError != nil {
				t.Errorf("Did not receive an error matching %v", test.expectedError)
				return
			}
			if err != nil && test.expectedError == nil {
				t.Error(err)
				return
			}
			if err != nil && !test.expectedError.MatchString(err.Error()) {
				t.Errorf("Expected error matching %v, got %v", test.expectedError, err)
				return
			}

			if len(mockRunner.expectedCommands) != 0 {
				t.Errorf("MockRunner: Unmatched expectedCommands %v", mockRunner.expectedCommands)
				return
			}
		})
	}
}

/* Integration test does not exercise normalizeRenderDirectories(), so add a unit test here */
func TestNormalizeRenderDirectories(t *testing.T) {
	t.Run("test directories are normalized", func(t *testing.T) {
		// Create tmpdir
		tmpDir, err := ioutil.TempDir(os.TempDir(), "render-test")
		if err != nil {
			t.Error(err)
			return
		}

		// Create some fake helmfile output directories
		paths := []string{
			path.Join(tmpDir, "dev", "helmfile-b47efc70-leonardo"),
			path.Join(tmpDir, "perf", "helmfile-b47efc70-leonardo"),
			path.Join(tmpDir, "alpha", "helmfile-b47efc70-cromwell"),
			path.Join(tmpDir, "alpha", "this-should-not-match"),
		}

		for _, path := range paths {
			if err = os.MkdirAll(path, 0755); err != nil {
				t.Error(err)
				return
			}
		}

		err = normalizeRenderDirectories(tmpDir)
		if err != nil {
			t.Error(err)
			return
		}

		// Paths above should have been renamed
		updatedPaths := []string{
			path.Join(tmpDir, "dev", "leonardo"),
			path.Join(tmpDir, "perf", "leonardo"),
			path.Join(tmpDir, "alpha", "cromwell"),
			path.Join(tmpDir, "alpha", "this-should-not-match"),
		}

		for _, path := range updatedPaths {
			if _, err := os.Stat(path); err != nil {
				t.Errorf("Expected path %s to exist: %v", path, err)
				return
			}
		}
	})
}

/*
Convenience function to generate tokenized argument list from format string w/ args

Eg. args("-e   %s", "dev") -> []string{"-e", "dev"}
*/
func args(format string, a ...interface{}) []string {
	formatted := fmt.Sprintf(format, a...)
	return strings.Fields(formatted)
}

/*
Convenience function to create a successful/non-erroring ExpectedCommand, given
a format string _for_ the command.

Eg. cmd("helmfile -e %s template", "alpha")

*/
func (ts *TestState) cmd(format string, a ...interface{}) ExpectedCommand {
	tokens := args(format, a...)

	return ExpectedCommand{
		Command: Command{
			Prog: tokens[0],
			Args: tokens[1:],
			Dir:  ts.mockConfigRepoPath,
		},
	}
}

/*
Convenience function to create a failing ExpectedCommand with an error
a format string _for_ the command.

Eg. cmd("helmfile -e %s template", "alpha")

*/
func (ts *TestState) failCmd(err string, format string, a ...interface{}) ExpectedCommand {
	expectedCommand := ts.cmd(format, a...)
	expectedCommand.MockError = errors.New(err)
	return expectedCommand
}

/* Set up a TestExecute test case */
func setup() (*TestState, error) {
	// Create a mock config repo clone in a tmp dir
	originalConfigRepoPath := os.Getenv(ConfigRepoPathEnvVar)

	tmpDir, err := ioutil.TempDir(os.TempDir(), "render-test")
	if err != nil {
		return nil, err
	}

	mockConfigRepoPath := path.Join(tmpDir, ConfigRepoName)
	err = os.MkdirAll(mockConfigRepoPath, 0755)
	if err != nil {
		return nil, err
	}

	// Create fake environment files
	err = createFakeEnvironmentFiles(mockConfigRepoPath, fakeEnvironments)
	if err != nil {
		return nil, err
	}

	// Overwrite TERRA_HELMFILE_PATH env var value with path to our fake config repo clone
	// Note: When Golang 1.17 is released we can use t.Setenv() instead https://github.com/golang/go/issues/41260
	err = os.Setenv(ConfigRepoPathEnvVar, mockConfigRepoPath)
	if err != nil {
		return nil, err
	}

	// Replace package-level default ShellRunner with a mock
	originalRunner := shellRunner
	mockRunner := &MockRunner{}
	shellRunner = mockRunner

	// Create mock chart dir inside tmp dir
	mockChartDir := path.Join(tmpDir, "charts")
	err = os.MkdirAll(mockChartDir, 0755)
	if err != nil {
		return nil, err
	}

	return &TestState{
		originalRunner:         &originalRunner,
		mockRunner:             mockRunner,
		originalConfigRepoPath: originalConfigRepoPath,
		mockConfigRepoPath:     mockConfigRepoPath,
		mockChartDir:           mockChartDir,
		tmpDir:                 tmpDir,
	}, nil
}

func cleanup(state *TestState) error {
	// Restore original ShellRunner
	shellRunner = *(state.originalRunner)

	// Restore original config repo path
	// When Golang 1.17 is released we can use t.Setenv() instead https://github.com/golang/go/issues/41260
	err := os.Setenv(ConfigRepoPathEnvVar, state.originalConfigRepoPath)
	if err != nil {
		return err
	}

	// Clean up temp dir
	return os.RemoveAll(state.tmpDir)
}

/* Create fake environment files like `environments/live/alpha.yaml` in mock config dir */
func createFakeEnvironmentFiles(mockConfigRepoPath string, envs []Environment) error {
	for _, env := range envs {
		baseDir := path.Join(mockConfigRepoPath, envSubdir, env.base)
		envFile := path.Join(baseDir, fmt.Sprintf("%s.yaml", env.name))

		err := os.MkdirAll(baseDir, 0755)
		if err != nil {
			return err
		}

		err = os.WriteFile(envFile, []byte("# Fake env file for testing"), 0644)
		if err != nil {
			return err
		}
	}

	return nil
}
