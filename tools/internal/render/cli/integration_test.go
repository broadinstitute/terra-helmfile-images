package cli

import (
	"errors"
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/render"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/render/helmfile"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/render/target"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock/matchers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

// This file contains an integration test for the render utility.
// It executes `render` with specific CLI arguments and verifies that the expected
// `helmfile` commands are executed under the hood.

// Fake environments and clusters, mocked for integration test
var devEnv = target.NewEnvironment("dev", "live")
var alphaEnv = target.NewEnvironment("alpha", "live")
var jdoeEnv = target.NewEnvironment("jdoe", "personal")
var perfCluster = target.NewCluster("terra-perf", "terra")
var tdrStagingCluster = target.NewCluster("tdr-staging", "tdr")

var fakeReleaseTargets = []target.ReleaseTarget{
	devEnv,
	alphaEnv,
	jdoeEnv,
	perfCluster,
	tdrStagingCluster,
}

// Struct for tracking global state that is mocked when a test executes and restored/cleaned up after
type TestState struct {
	mockRunner             *shellmock.MockRunner // mock shell.Runner, used for mocking shell commands
	originalRunner         shell.Runner          // real shell.Runner, saved before test starts and restored after they finish
	mockConfigRepoPath     string                // mock terra-helmfile, created once before all test cases
	originalConfigRepoPath string                // real config repo path, saved before tests start and restored after they finish
	mockChartDir           string                // mock chart directory, created once before all test cases
	scratchDir             string                // scratch directory, cleaned out before each test case
	rootDir                string                // root/parent directory for all test files
}

// A table-driven integration test for the render tool.
//
// Given a list of CLI arguments to the render Cobra command, the test verifies
// that the correct underlying `helmfile` command(s) are run.
//
// Reference:
// https://gianarb.it/blog/golang-mockmania-cli-command-with-cobra
func TestRender(t *testing.T) {
	var testCases = []struct {
		description   string                                // Testcase description
		arguments     []string                              // Fake user-supplied CLI arguments to pass to `render`
		argumentsFn   func(ts *TestState) ([]string, error) // Callback function returning fake user-supplied CLI arguments to pass to `render`. Will override `arguments` field if given
		expectedError *regexp.Regexp                        // Optional error we expect to be returned when we execute the Cobra command
		setupMocks    func(ts *TestState) error             // Optional hook mocking expectedAttrs shell commands
		verifyFn      func(ts *TestState, t *testing.T)     // Optional hook for verifying results are as expectedAttrs
	}{
		{
			description: "unknown environment should return error",
			arguments:   args("-e foo"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				return nil
			},
			expectedError: regexp.MustCompile("unknown environment: foo"),
		},
		{
			description: "unknown cluster should return error",
			arguments:   args("-c blargh"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				return nil
			},
			expectedError: regexp.MustCompile("unknown cluster: blargh"),
		},
		{
			description: "no arguments should render for all targets",
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(tdrStagingCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(perfCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(alphaEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/alpha", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(devEnv, " --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(jdoeEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/jdoe", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "--parallel-workers=10 should render without errors",
			arguments:   args("--parallel-workers=10"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(tdrStagingCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(perfCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(alphaEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/alpha", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(devEnv, " --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(jdoeEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/jdoe", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "--argocd without -e or -a should render Argo manifests for all targets",
			arguments:   args("--argocd"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(tdrStagingCluster, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(perfCluster, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(alphaEnv, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/alpha", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				ts.expectHelmfileCmd(jdoeEnv, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/jdoe", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-e should render for specific environment",
			arguments:   args("-e dev"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-c should render for specific cluster",
			arguments:   args("-c terra-perf"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(perfCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-e with --argocd should render ArgoCD manifests for specific environment",
			arguments:   args("-e dev --argocd"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-c with --argocd should render ArgoCD manifests for specific cluster",
			arguments:   args("-c tdr-staging --argocd"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(tdrStagingCluster, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r should render for specific service",
			arguments:   args("-e dev -r leonardo"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with --argocd should render ArgoCD manifests for specific service",
			arguments:   args("--argocd -e dev -a leonardo"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=argocd,release=leonardo template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with --app-version should set app version",
			arguments:   args("-e dev -r leonardo --app-version 1.2.3"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.appVersion=1.2.3 template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with --chart-dir should set chart dir and not include --skip-deps",
			argumentsFn: func(ts *TestState) ([]string, error) {
				return args("-e dev -r leonardo --chart-dir=%s", ts.mockChartDir), nil
			},
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.repo=%s template --output-dir=%s/output/dev", ts.mockChartDir, ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with --app-version and --chart-version should set both",
			arguments:   args("-e dev -r leonardo --app-version 1.2.3 --chart-version 4.5.6"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.appVersion=1.2.3,releases.leonardo.chartVersion=4.5.6 template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with --values-file should set values file",
			argumentsFn: func(ts *TestState) ([]string, error) {
				valuesFile, err := ts.createScratchFile("v.yaml", "# fake values file")
				if err != nil {
					return nil, err
				}
				return args("-e dev -r leonardo --values-file %s", valuesFile), nil
			},
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo template --skip-deps --values=%s/v.yaml --output-dir=%s/output/dev", ts.scratchDir, ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with multiple --values-file should set values files in order",
			argumentsFn: func(ts *TestState) ([]string, error) {
				valuesFiles, err := ts.createScratchFiles("# fake values file", "v1.yaml", "v2.yaml", "v3.yaml")
				if err != nil {
					return nil, err
				}
				return args("-e dev -r leonardo --values-file %s --values-file %s --values-file %s", valuesFiles[0], valuesFiles[1], valuesFiles[2]), nil
			},
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo template --skip-deps --values=%s/v1.yaml,%s/v2.yaml,%s/v3.yaml --output-dir=%s/output/dev", ts.scratchDir, ts.scratchDir, ts.scratchDir, ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "should fail if repo update fails",
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd().Return(errors.New("dieee"))
				return nil
			},
			expectedError: regexp.MustCompile("dieee"),
		},
		{
			description: "should fail if helmfile template fails",
			arguments:   args("-e dev"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath).Return(errors.New("dieee"))
				return nil
			},
			expectedError: regexp.MustCompile("dieee"),
		},
		{
			description: "should run helmfile with --log-level=debug if run with -v -v",
			arguments:   args("-e dev -v -v"),
			setupMocks: func(ts *TestState) error {
				ts.cmd("helmfile --log-level=debug --allow-no-matching-release repos")
				ts.expectHelmfileCmd(devEnv, "--log-level=debug --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "--stdout should not render to output directory",
			arguments:   args("--env=alpha --stdout"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(alphaEnv, "--log-level=info --selector=mode=release template --skip-deps")
				return nil
			},
		},
		{
			description: "-d should render to custom output directory",
			arguments:   args("-e jdoe -d path/to/nowhere"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(jdoeEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/path/to/nowhere/jdoe", cwd())
				return nil
			},
		},
		{
			description: "--scratch-dir should be passed to helmfile if it does exist",
			argumentsFn: func(ts *TestState) ([]string, error) {
				dir := path.Join(ts.scratchDir, "user-supplied")
				if err := os.MkdirAll(dir, 0755); err != nil {
					return nil, err
				}
				return args("-e dev -a leonardo --scratch-dir %s", dir), nil
			},
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()

				matcher := helmfileCmdMatcher(devEnv, "--log-level=info --selector=mode=release,release=leonardo template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				matcher.WithEnvVar(helmfile.ChartCacheDirEnvVar, matchers.Equals(path.Join(ts.scratchDir, "user-supplied", "chart-cache")))
				ts.mockRunner.ExpectCmd(matcher)

				return nil
			},
		},
		{
			description: "two environments with the same name should raise an error",
			setupMocks: func(ts *TestState) error {
				return createFakeTargetFiles(ts.mockConfigRepoPath, []target.ReleaseTarget{target.NewEnvironmentGeneric("dev", "personal")})
			},
			expectedError: regexp.MustCompile(`environment name conflict dev \(personal\) and dev \(live\)`),
		},
		{
			description: "two clusters with the same name should raise an error",
			setupMocks: func(ts *TestState) error {
				return createFakeTargetFiles(ts.mockConfigRepoPath, []target.ReleaseTarget{target.NewClusterGeneric("terra-perf", "tdr")})
			},
			expectedError: regexp.MustCompile(`cluster name conflict terra-perf \(terra\) and terra-perf \(tdr\)`),
		},
		{
			description: "environment and cluster with the same name should raise an error",
			setupMocks: func(ts *TestState) error {
				return createFakeTargetFiles(ts.mockConfigRepoPath, []target.ReleaseTarget{target.NewClusterGeneric("dev", "terra")})
			},
			expectedError: regexp.MustCompile("cluster name dev conflicts with environment name dev"),
		},
		{
			description: "missing config directory should raise an error",
			setupMocks: func(ts *TestState) error {
				return os.RemoveAll(ts.mockConfigRepoPath)
			},
			expectedError: regexp.MustCompile("config repo clone does not exist"),
		},
		{
			description: "missing environments directory should raise an error",
			setupMocks: func(ts *TestState) error {
				return os.RemoveAll(path.Join(ts.mockConfigRepoPath, "environments"))
			},
			expectedError: regexp.MustCompile("environment config directory does not exist"),
		},
		{
			description: "missing clusters directory should raise an error",
			setupMocks: func(ts *TestState) error {
				return os.RemoveAll(path.Join(ts.mockConfigRepoPath, "clusters"))
			},
			expectedError: regexp.MustCompile("cluster config directory does not exist"),
		},
		{
			description: "no environment definitions should raise an error",
			setupMocks: func(ts *TestState) error {
				envDir := path.Join(ts.mockConfigRepoPath, "environments")
				if err := os.RemoveAll(envDir); err != nil {
					return err
				}
				return os.MkdirAll(envDir, 0755)
			},
			expectedError: regexp.MustCompile("no environment configs found"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			// Run pre test-case setup
			ts, err := setup(t)
			if err != nil {
				t.Error(err)
				return
			}

			// Set up mocks for this test case's commands
			if testCase.setupMocks != nil {
				if err := testCase.setupMocks(ts); err != nil {
					t.Errorf("setupMocks error: %v", err)
				}
			}

			// Get arguments to pass to Cobra command
			cliArgs := testCase.arguments
			if testCase.argumentsFn != nil {
				cliArgs, err = testCase.argumentsFn(ts)
				if err != nil {
					t.Errorf("argumentsFn error: %v", err)
					return
				}
			}

			// Run the Cobra command
			cli := newCLI(false)
			cli.setArgs(cliArgs)
			err = cli.execute()

			// Verify error matches expectations
			if testCase.expectedError == nil {
				if !assert.NoError(t, err, "Unexpected error returned: %v", err) {
					return
				}
			} else {
				if !assert.Error(t, err, "Expected command execution to return an error, but it did not") {
					return
				}
				assert.Regexp(t, testCase.expectedError, err.Error(), "Error mismatch")
			}

			// Verify all expectedAttrs commands were run
			ts.mockRunner.AssertExpectations(t)
		})
	}
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

// Convenience function for setting up an expectation for a helmfile update command
func (ts *TestState) expectHelmfileUpdateCmd() *mock.Call {
	return ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
}

// Convenience function for setting up an expectation for a helmfile template command
func (ts *TestState) expectHelmfileCmd(target target.ReleaseTarget, format string, a ...interface{}) *mock.Call {
	matcher := helmfileCmdMatcher(target, format, a...)
	return ts.mockRunner.ExpectCmd(matcher)
}

// Given a release target, and CLI arguments to `helmfile` in the form of a format string and arguments,
// return a matcher for a `helmfile` command that matches the given target and CLI args
func helmfileCmdMatcher(target target.ReleaseTarget, format string, a ...interface{}) *matchers.CmdMatcher {
	directoryExists := matchers.MatchesPredicate("directory exists", func(dir string) bool {
		f, err := os.Stat(dir)
		if err != nil {
			return false
		}
		return f.IsDir()
	})

	matcher := matchers.CmdWithProg(helmfile.Command).
		WithEnvVar(helmfile.TargetTypeEnvVar, target.Type()).
		WithEnvVar(helmfile.TargetBaseEnvVar, target.Base()).
		WithEnvVar(helmfile.TargetNameEnvVar, target.Name()).
		WithEnvVar(helmfile.ChartCacheDirEnvVar, directoryExists).
		WithExactArgs(args(format, a...)...)

	return matcher
}

// Convenience function to create a successful/non-erroring ExpectedCommand, given
// a format string _for_ the command.
//
// Eg. cmd("THF_TARGET_NAME=%s FOO=BAR helmfile template", "alpha")
// ->
// Command{
//   Env: []string{"THF_TARGET_NAME=alpha", "FOO=BAR"},
//   Prog: "helmfile",
//   Args: []string{"template"},
//   Dir: "mock/config/repo/path" // tmpdir
// }
func (ts *TestState) cmd(format string, a ...interface{}) *mock.Call {
	tokens := args(format, a...)

	// count number of leading NAME=VALUE environment var pairs preceding `helmfile` command
	var i int
	for i = 0; i < len(tokens); i++ {
		if !strings.Contains(tokens[i], "=") {
			// if this is not a NAME=VALUE pair, quit
			break
		}
	}

	cmd := shell.Command{
		Env:  tokens[0:i],
		Prog: tokens[i],
		Args: tokens[i+1:],
		Dir:  ts.mockConfigRepoPath,
	}

	return ts.mockRunner.ExpectCmd(cmd)
}

// Per-test setup, run before each TestRender test case
func setup(t *testing.T) (*TestState, error) {
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

	// Create mock chart dir inside tmp dir
	mockChartDir := path.Join(tmpDir, "charts")
	err = os.MkdirAll(mockChartDir, 0755)
	if err != nil {
		return nil, err
	}

	// Create mock environment and cluster target files
	if err := createFakeTargetFiles(mockConfigRepoPath, fakeReleaseTargets); err != nil {
		return nil, err
	}

	// Create scratch directory, cleaned after every test case.
	scratchDir := path.Join(tmpDir, "scratch")

	// Create a mock runner for executing shell commands
	mockRunner := shellmock.NewMockRunner(shellmock.Options{VerifyOrder: false})
	mockRunner.Test(t)

	// Replace real shell runner with mock runner; will be restored by cleanup() when this test completes
	originalRunner := render.SetRunner(mockRunner)

	ts := &TestState{
		originalConfigRepoPath: originalConfigRepoPath,
		mockConfigRepoPath:     mockConfigRepoPath,
		originalRunner:         originalRunner,
		mockRunner:             mockRunner,
		mockChartDir:           mockChartDir,
		scratchDir:             scratchDir,
		rootDir:                tmpDir,
	}

	// Add cleanup hook to clean up tmp directories & restore global state after tests complete
	t.Cleanup(func() {
		err := cleanup(ts)
		if err != nil {
			t.Error(err)
		}
	})

	return ts, nil
}

// One-time cleanup, run after all TestCases have run
func cleanup(state *TestState) error {
	// Restore original config repo path
	// When Golang 1.17 is released we can use t.Setenv() instead https://github.com/golang/go/issues/41260
	err := os.Setenv(configRepoPathEnvVar, state.originalConfigRepoPath)
	if err != nil {
		return err
	}

	// Restore real shell runner
	render.SetRunner(state.originalRunner)

	// Clean up temp dir
	return os.RemoveAll(state.rootDir)
}

// Create fake target files like `environments/live/alpha.yaml` and `clusters/terra/terra-dev.yaml` in mock config dir
func createFakeTargetFiles(mockConfigRepoPath string, targets []target.ReleaseTarget) error {
	for _, releaseTarget := range targets {
		baseDir := path.Join(mockConfigRepoPath, releaseTarget.ConfigDir(), releaseTarget.Base())
		configFile := path.Join(baseDir, fmt.Sprintf("%s.yaml", releaseTarget.Name()))

		if err := createFile(configFile, "# Fake file for mock"); err != nil {
			return err
		}
	}

	return nil
}

// Convenience function for creating multiple fake files in scratch directory
func (ts *TestState) createScratchFiles(content string, filenames ...string) ([]string, error) {
	filepaths := make([]string, len(filenames))
	for i, f := range filenames {
		filepath, err := ts.createScratchFile(f, content)
		if err != nil {
			return nil, err
		}
		filepaths[i] = filepath
	}
	return filepaths, nil
}

// Convenience function for creating a fake file in scratch directory
func (ts *TestState) createScratchFile(filename string, content string) (filepath string, err error) {
	filepath = path.Join(ts.scratchDir, filename)
	err = createFile(filepath, content)
	return
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