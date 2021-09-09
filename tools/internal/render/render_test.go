package render

import (
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

// This file contains an integration test for the render utility

// Fake environments and clusters, mocked for integration testing
var fakeReleaseTargets = []ReleaseTarget{
	NewEnvironment("dev", "live"),
	NewEnvironment("alpha", "live"),
	NewEnvironment("jdoe", "personal"),
	NewCluster("terra-perf", "terra"),
	NewCluster("tdr-staging", "tdr"),
}

// Struct for tracking global state that is mocked when a test executes and restored/cleaned up after
type TestState struct {
	mockRunner             *MockRunner  // mock ShellRunner, reset before every test case
	mockConfigRepoPath     string       // mock terra-helmfile, created once before all test cases
	mockChartDir           string       // mock chart directory, created once before all test cases
	scratchDir             string       // scratch directory, cleaned out before each test case
	rootDir                string       // root/parent directory for all test files
	originalRunner         *ShellRunner // real ShellRunner, saved before tests start and restored after they finish
	originalConfigRepoPath string       // real config repo path, saved before tests start and restored after they finish
}

type TestCase struct {
	description      string            // Testcase description
	arguments        []string          // Fake user-supplied CLI arguments to pass to `render`
	expectedCommands []ExpectedCommand // Ordered list of CLI commands we expect `render` to run
	expectedError    *regexp.Regexp    // Optional error we expect to be raised
	setup            func() error      // Optional hook for extra setup
}

// Used in MockRunner to verify expected commands have been called
type ExpectedCommand struct {
	Command   Command // Expected Command
	MockError error   // Optional error to return (for faking an error running the command)
}

// MockRunner stores an ordered list (slice) of expected commands
type MockRunner struct {
	expectedCommands []ExpectedCommand // Ordered list of expected commands
}

// Mock implementation of ShellCommand
//
// Instead of executing the command, compare it to the runner's list of expected commands,
// throwing an error on mismatch
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

// A table-driven integration test for the render tool.
//
// Given a list of CLI arguments to the render command, the test verifies
// that the correct underlying `helmfile` command(s) are run.
//
// Reference:
// https://gianarb.it/blog/golang-mockmania-cli-command-with-cobra
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

	var tests = []TestCase{
		{
			description:   "invalid argument",
			arguments:     args("--foo"),
			expectedError: regexp.MustCompile("unknown flag"),
		},
		{
			description:   "-a should require -e or -c",
			arguments:     args("-a foo"),
			expectedError: regexp.MustCompile(`an environment \(-e\) or cluster \(-c\) must be specified when a release is specified with -r`),
		},
		{
			description:   "-r should require -e or -c",
			arguments:     args("-r foo"),
			expectedError: regexp.MustCompile(`an environment \(-e\) or cluster \(-c\) must be specified when a release is specified with -r`),
		},
		{
			description:   "-e and -c incompatible",
			arguments:     args("-c terra-perf -e dev"),
			expectedError: regexp.MustCompile("only one of -e/--environment and -c/--cluster may be specified"),
		},
		{
			description:   "--app-version should require -r",
			arguments:     args("--app-version 1.0.0"),
			expectedError: regexp.MustCompile("--app-version requires a release be specified with -r"),
		},
		{
			description:   "--chart-version should require -r",
			arguments:     args("--chart-version 1.0.0"),
			expectedError: regexp.MustCompile("--chart-version requires a release be specified with -r"),
		},
		{
			description:   "--chart-dir should require -r",
			arguments:     args("--chart-dir %s", ts.mockChartDir),
			expectedError: regexp.MustCompile("--chart-dir requires a release be specified with -r"),
		},
		{
			description:   "--values-file should require -r",
			arguments:     args("--values-file %s", path.Join(ts.rootDir, "missing.yaml")),
			expectedError: regexp.MustCompile("--values-file requires a release be specified with -r"),
		},
		{
			description:   "--values-file must exist",
			arguments:     args("-e dev -r leonardo --values-file %s", path.Join(ts.rootDir, "missing.yaml")),
			expectedError: regexp.MustCompile("values file does not exist: .*/missing.yaml"),
		},
		{
			description:   "--chart-dir and --chart-version incompatible",
			arguments:     args("-e dev -r leonardo --chart-dir %s --chart-version 1.0.0", ts.mockChartDir),
			expectedError: regexp.MustCompile("only one of --chart-dir or --chart-version may be specified"),
		},
		{
			description:   "--chart-dir must exist",
			arguments:     args("-e dev -r leonardo --chart-dir path/to/nowhere"),
			expectedError: regexp.MustCompile("chart directory does not exist: .*/path/to/nowhere"),
		},
		{
			description:   "--argocd and --app-version incompatible",
			arguments:     args("-e dev -r leonardo --app-version 1.0.0 --argocd"),
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--app-version"),
		},
		{
			description:   "--argocd and --chart-version incompatible",
			arguments:     args("-e dev -r leonardo --chart-version 1.0.0 --argocd"),
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--chart-version"),
		},
		{
			description:   "--argocd and --chart-dir incompatible",
			arguments:     args("-e dev -r leonardo --chart-dir=%s --argocd", ts.mockChartDir),
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--chart-dir"),
		},
		{
			description:   "--argocd and --values-file incompatible",
			arguments:     args("-e dev -r leonardo --values-file=%s --argocd", "missing.yaml"),
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--values-file"),
		},
		{
			description:   "--stdout and --output-dir incompatible",
			arguments:     args("-e dev -r leonardo -d /tmp/output --stdout"),
			expectedError: regexp.MustCompile("--stdout cannot be used with -d/--output-dir"),
		},
		{
			description:   "--cluster and --app-version incompatible",
			arguments:     args("--cluster terra-perf -r leonardo --app-version=0.0.1"),
			expectedError: regexp.MustCompile("--app-version cannot be used for cluster releases"),
		},
		{
			description: "unknown environment should return error",
			arguments:   args("-e foo"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
			},
			expectedError: regexp.MustCompile("unknown environment: foo"),
		},
		{
			description: "unknown cluster should return error",
			arguments:   args("-c blargh"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
			},
			expectedError: regexp.MustCompile("unknown cluster: blargh"),
		},
		{
			description: "no arguments should render for all targets",
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=cluster     TARGET_BASE=tdr      TARGET_NAME=tdr-staging helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath),
				ts.cmd("TARGET_TYPE=cluster     TARGET_BASE=terra    TARGET_NAME=terra-perf  helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live     TARGET_NAME=alpha       helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/alpha", ts.mockConfigRepoPath),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live     TARGET_NAME=dev         helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=personal TARGET_NAME=jdoe        helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/jdoe", ts.mockConfigRepoPath),
			},
		},
		{
			description: "--argocd without -e or -a should render Argo manifests for all environments",
			arguments:   args("--argocd"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=cluster     TARGET_BASE=tdr      TARGET_NAME=tdr-staging helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath),
				ts.cmd("TARGET_TYPE=cluster     TARGET_BASE=terra    TARGET_NAME=terra-perf  helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live     TARGET_NAME=alpha       helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/alpha", ts.mockConfigRepoPath),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live     TARGET_NAME=dev         helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=personal TARGET_NAME=jdoe        helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/jdoe", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-e should render for specific environment",
			arguments:   args("-e dev"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-c should render for specific cluster",
			arguments:   args("-c terra-perf"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=cluster TARGET_BASE=terra TARGET_NAME=terra-perf helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-e with --argocd should render ArgoCD manifests for specific environment",
			arguments:   args("-e dev --argocd"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-c with --argocd should render ArgoCD manifests for specific cluster",
			arguments:   args("-c tdr-staging --argocd"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=cluster TARGET_BASE=tdr TARGET_NAME=tdr-staging helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-r should render for specific service",
			arguments:   args("-e dev -r leonardo"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-r with --argocd should render ArgoCD manifests for specific service",
			arguments:   args("--argocd -e dev -a leonardo"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=argocd,release=leonardo template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-r with --app-version should set app version",
			arguments:   args("-e dev -r leonardo --app-version 1.2.3"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.appVersion=1.2.3 template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-r with --chart-dir should set chart dir and not include --skip-deps",
			arguments:   args("-e dev -r leonardo --chart-dir=%s", ts.mockChartDir),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.repo=%s template --output-dir=%s/output/dev", ts.mockChartDir, ts.mockConfigRepoPath),
			},
		},
		{
			description: "-r with --app-version and --chart-version should set both",
			arguments:   args("-e dev -r leonardo --app-version 1.2.3 --chart-version 4.5.6"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.appVersion=1.2.3,releases.leonardo.chartVersion=4.5.6 template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "-r with --values-file should set values file",
			arguments:   args(" -e dev -r leonardo --values-file %s/v.yaml", ts.scratchDir),
			setup: func() error {
				return ts.createScratchFile("v.yaml", "# fake values file")
			},
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo template --skip-deps --values=%s/v.yaml --output-dir=%s/output/dev", ts.scratchDir, ts.mockConfigRepoPath),
			},
		},
		{
			description: "-r with multiple --values-file should set values files in order",
			arguments:   args("-e dev -r leonardo --values-file %s/v1.yaml --values-file %s/v2.yaml --values-file %s/v3.yaml", ts.scratchDir, ts.scratchDir, ts.scratchDir),
			setup: func() error {
				return ts.createScratchFiles("# fake values file", "v1.yaml", "v2.yaml", "v3.yaml")
			},
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo template --skip-deps --values=%s/v1.yaml,%s/v2.yaml,%s/v3.yaml --output-dir=%s/output/dev", ts.scratchDir, ts.scratchDir, ts.scratchDir, ts.mockConfigRepoPath),
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
				ts.failCmd("dieee", "TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
			expectedError: regexp.MustCompile("dieee"),
		},
		{
			description: "should run helmfile with --log-level=debug if run with -v -v",
			arguments:   args("-e dev -v -v"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=debug --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=dev helmfile --log-level=debug --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath),
			},
		},
		{
			description: "--stdout should not render to output directory",
			arguments:   args("--env=alpha --stdout"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=live TARGET_NAME=alpha helmfile --log-level=info --selector=mode=release template --skip-deps"),
			},
		},
		{
			description: "-d should render to custom output directory",
			arguments:   args("-e jdoe -d path/to/nowhere"),
			expectedCommands: []ExpectedCommand{
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos"),
				ts.cmd("TARGET_TYPE=environment TARGET_BASE=personal TARGET_NAME=jdoe helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/path/to/nowhere/jdoe", cwd()),
			},
		},
		{
			description: "two environments with the same name should raise an error",
			setup: func() error {
				return createFakeTargetFiles(ts.mockConfigRepoPath, []ReleaseTarget{NewEnvironmentGeneric("dev", "personal")})
			},
			expectedError: regexp.MustCompile(`environment name conflict dev \(personal\) and dev \(live\)`),
		},
		{
			description: "two clusters with the same name should raise an error",
			setup: func() error {
				return createFakeTargetFiles(ts.mockConfigRepoPath, []ReleaseTarget{NewClusterGeneric("terra-perf", "tdr")})
			},
			expectedError: regexp.MustCompile(`cluster name conflict terra-perf \(terra\) and terra-perf \(tdr\)`),
		},
		{
			description: "environment and cluster with the same name should raise an error",
			setup: func() error {
				return createFakeTargetFiles(ts.mockConfigRepoPath, []ReleaseTarget{NewClusterGeneric("dev", "terra")})
			},
			expectedError: regexp.MustCompile("cluster name dev conflicts with environment name dev"),
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if err := ts.setupTestCase(test); err != nil {
				t.Error(err)
				return
			}

			cobraCmd := newCobraCommand()
			cobraCmd.SetArgs(test.arguments)
			err := cobraCmd.Execute()

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

			if len(ts.mockRunner.expectedCommands) != 0 {
				t.Errorf("MockRunner: Unmatched expectedCommands %v", ts.mockRunner.expectedCommands)
				return
			}
		})
	}
}

// Integration test does not exercise normalizeRenderDirectories(), so add a unit test here
func TestNormalizeRenderDirectories(t *testing.T) {
	t.Run("test directories are normalized", func(t *testing.T) {
		// Create tmpdir
		tmpDir, err := ioutil.TempDir(os.TempDir(), "render-test")
		if err != nil {
			t.Error(err)
			return
		}

		// Create some fake helmfile output directories
		manifestDirs := []string{
			path.Join(tmpDir, "dev", "helmfile-b47efc70-leonardo"),
			path.Join(tmpDir, "perf", "helmfile-b47efc70-leonardo"),
			path.Join(tmpDir, "alpha", "helmfile-b47efc70-cromwell"),
			path.Join(tmpDir, "alpha", "this-should-not-match"),
		}

		for _, dir := range manifestDirs {
			if err = os.MkdirAll(dir, 0755); err != nil {
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
		renamedDirs := []string{
			path.Join(tmpDir, "dev", "leonardo"),
			path.Join(tmpDir, "perf", "leonardo"),
			path.Join(tmpDir, "alpha", "cromwell"),
			path.Join(tmpDir, "alpha", "this-should-not-match"),
		}

		for _, dir := range renamedDirs {
			if _, err := os.Stat(dir); err != nil {
				t.Errorf("Expected path %s to exist: %v", dir, err)
				return
			}
		}
	})
}

// Convenience function to generate tokenized argument list from format string w/ args
//
// Eg. args("-e   %s", "dev") -> []string{"-e", "dev"}
func args(format string, a ...interface{}) []string {
	formatted := fmt.Sprintf(format, a...)
	return strings.Fields(formatted)
}

// Convenience function to return current working directory
func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return dir
}

// Convenience function to create a successful/non-erroring ExpectedCommand, given
// a format string _for_ the command.
//
// Eg. cmd("TARGET_NAME=%s FOO=BAR helmfile template", "alpha")
// ->
// Command{
//   Env: []string{"TARGET_NAME=alpha", "FOO=BAR"},
//   Prog: "helmfile",
//   Args: []string{"template"},
//   Dir: "mock/config/repo/path" // tmpdir
// }
func (ts *TestState) cmd(format string, a ...interface{}) ExpectedCommand {
	tokens := args(format, a...)

	// count number of leading NAME=VALUE environment var pairs preceding `helmfile` command
	var i int
	for i = 0; i < len(tokens); i++ {
		if !strings.Contains(tokens[i], "=") {
			// if this is not a NAME=VALUE pair, quit
			break
		}
	}

	return ExpectedCommand{
		Command: Command{
			Env:  tokens[0:i],
			Prog: tokens[i],
			Args: tokens[i+1:],
			Dir:  ts.mockConfigRepoPath,
		},
	}
}

// Convenience function to create a failing ExpectedCommand with an error
// a format string _for_ the command.
//
// Eg. cmd("helmfile -e %s template", "alpha")
func (ts *TestState) failCmd(err string, format string, a ...interface{}) ExpectedCommand {
	expectedCommand := ts.cmd(format, a...)
	expectedCommand.MockError = errors.New(err)
	return expectedCommand
}

// Per-test case setup
func (ts *TestState) setupTestCase(tc TestCase) error {
	// Delete and re-create scratch directory
	if err := os.RemoveAll(ts.scratchDir); err != nil {
		return err
	}
	if err := os.MkdirAll(ts.scratchDir, 0755); err != nil {
		return err
	}

	// Create fake environment files
	if err := os.RemoveAll(ts.mockConfigRepoPath); err != nil {
		return err
	}
	if err := os.MkdirAll(ts.mockConfigRepoPath, 0755); err != nil {
		return err
	}

	if err := createFakeTargetFiles(ts.mockConfigRepoPath, fakeReleaseTargets); err != nil {
		return err
	}

	// Execute setup callback function if one was given
	if tc.setup != nil {
		if err := tc.setup(); err != nil {
			return fmt.Errorf("setup error: %v", err)
		}
	}

	// Set expectedCommands to the test-case's expected commands
	ts.mockRunner.expectedCommands = tc.expectedCommands

	return nil
}

// One-time setup, run before all TestRender test cases
func setup() (*TestState, error) {
	// Create a mock config repo clone in a tmp dir
	originalConfigRepoPath := os.Getenv(configRepoPathEnvVar)

	tmpDir, err := ioutil.TempDir(os.TempDir(), "render-test")
	if err != nil {
		return nil, err
	}

	mockConfigRepoPath := path.Join(tmpDir, configRepoName)
	err = os.MkdirAll(mockConfigRepoPath, 0755)
	if err != nil {
		return nil, err
	}

	// Overwrite TERRA_HELMFILE_PATH env var value with path to our fake config repo clone
	// Note: When Golang 1.17 is released we can use t.Setenv() instead https://github.com/golang/go/issues/41260
	err = os.Setenv(configRepoPathEnvVar, mockConfigRepoPath)
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

	// Create scratch directory, cleaned after every test case.
	scratchDir := path.Join(tmpDir, "scratch")

	return &TestState{
		originalRunner:         &originalRunner,
		mockRunner:             mockRunner,
		originalConfigRepoPath: originalConfigRepoPath,
		mockConfigRepoPath:     mockConfigRepoPath,
		mockChartDir:           mockChartDir,
		scratchDir:             scratchDir,
		rootDir:                tmpDir,
	}, nil
}

// One-time cleanup, run after all TestCases have run
func cleanup(state *TestState) error {
	// Restore original ShellRunner
	shellRunner = *(state.originalRunner)

	// Restore original config repo path
	// When Golang 1.17 is released we can use t.Setenv() instead https://github.com/golang/go/issues/41260
	err := os.Setenv(configRepoPathEnvVar, state.originalConfigRepoPath)
	if err != nil {
		return err
	}

	// Clean up temp dir
	return os.RemoveAll(state.rootDir)
}

// Create fake target files like `environments/live/alpha.yaml` and `clusters/terra/terra-dev.yaml` in mock config dir
func createFakeTargetFiles(mockConfigRepoPath string, targets []ReleaseTarget) error {
	for _, target := range targets {
		baseDir := path.Join(mockConfigRepoPath, target.ConfigDir(), target.Base())
		configFile := path.Join(baseDir, fmt.Sprintf("%s.yaml", target.Name()))

		if err := createFile(configFile, "# Fake file for testing"); err != nil {
			return err
		}
	}

	return nil
}

// Convenience function for creating multiple fake files in scratch directory
func (ts *TestState) createScratchFiles(content string, filenames ...string) error {
	for _, f := range filenames {
		if err := ts.createScratchFile(f, content); err != nil {
			return err
		}
	}
	return nil
}

// Convenience function for creating a fake file in scratch directory
func (ts *TestState) createScratchFile(filename string, content string) error {
	return createFile(path.Join(ts.scratchDir, filename), content)
}

// Convenience function for creating a fake file
func createFile(filepath string, content string) error {
	dir := path.Dir(filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return err
	}

	return nil
}
