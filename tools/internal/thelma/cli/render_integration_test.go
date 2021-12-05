package cli

import (
	"errors"
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/helmfile"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	. "github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/testutils"
	"github.com/stretchr/testify/assert"
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
var devEnv = gitops.NewEnvironment("dev", "live", "TODO", nil)
var alphaEnv = gitops.NewEnvironment("alpha", "live", "TODO", nil)
var jdoeEnv = gitops.NewEnvironment("jdoe", "personal", "TODO", nil)

var perfCluster = gitops.NewCluster("terra-perf", "terra", "TODO", nil)
var tdrStagingCluster = gitops.NewCluster("tdr-staging", "tdr", "TODO", nil)

var fakeReleaseTargets = []gitops.Target{
	devEnv,
	alphaEnv,
	jdoeEnv,
	perfCluster,
	tdrStagingCluster,
}

// Struct for tracking global state that is mocked when a test executes and restored/cleaned up after
type TestState struct {
	mockRunner      *shell.MockRunner // mock shell.Runner, used for mocking shell commands
	mockHome        string            // mock terra-helmfile clone, created once before all test cases
	mockChartSrcDir string            // mock chart source directory, created once before all test cases
	scratchDir      string            // scratch directory, cleaned out before each test case
	thelmaCLI       *ThelmaCLI        // thelmaCLI wired with the above
}

// A table-driven integration test for the render tool.
//
// Given a list of CLI arguments to the render Cobra command, the test verifies
// that the correct underlying `helmfile` command(s) are run.
//
// Reference:
// https://gianarb.it/blog/golang-mockmania-cli-command-with-cobra
func TestRenderIntegration(t *testing.T) {
	t.Skip("TODO")
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
			arguments:   args("render -e foo"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				return nil
			},
			expectedError: regexp.MustCompile("unknown environment: foo"),
		},
		{
			description: "unknown cluster should return error",
			arguments:   args("render -c blargh"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				return nil
			},
			expectedError: regexp.MustCompile("unknown cluster: blargh"),
		},
		{
			description: "no arguments should render for all targets",
			arguments:   args("render"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(tdrStagingCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockHome)
				ts.expectHelmfileCmd(perfCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockHome)
				ts.expectHelmfileCmd(alphaEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/alpha", ts.mockHome)
				ts.expectHelmfileCmd(devEnv, " --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
				ts.expectHelmfileCmd(jdoeEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/jdoe", ts.mockHome)
				return nil
			},
		},
		{
			description: "--parallel-workers=10 should render without errors",
			arguments:   args("render --parallel-workers=10"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(tdrStagingCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockHome)
				ts.expectHelmfileCmd(perfCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockHome)
				ts.expectHelmfileCmd(alphaEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/alpha", ts.mockHome)
				ts.expectHelmfileCmd(devEnv, " --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
				ts.expectHelmfileCmd(jdoeEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/jdoe", ts.mockHome)
				return nil
			},
		},
		{
			description: "--argocd without -e or -a should render Argo manifests for all targets",
			arguments:   args("render --argocd"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(tdrStagingCluster, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockHome)
				ts.expectHelmfileCmd(perfCluster, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/terra-perf", ts.mockHome)
				ts.expectHelmfileCmd(alphaEnv, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/alpha", ts.mockHome)
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
				ts.expectHelmfileCmd(jdoeEnv, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/jdoe", ts.mockHome)
				return nil
			},
		},
		{
			description: "-e should render for specific environment",
			arguments:   args("render -e dev"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
				return nil
			},
		},
		{
			description: "-c should render for specific cluster",
			arguments:   args("render -c terra-perf"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(perfCluster, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockHome)
				return nil
			},
		},
		{
			description: "-e with --argocd should render ArgoCD manifests for specific environment",
			arguments:   args("render -e dev --argocd"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
				return nil
			},
		},
		{
			description: "-c with --argocd should render ArgoCD manifests for specific cluster",
			arguments:   args("render -c tdr-staging --argocd"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(tdrStagingCluster, "--log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockHome)
				return nil
			},
		},
		{
			description: "-r should render for specific service",
			arguments:   args("render -e dev -r leonardo"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
				return nil
			},
		},
		{
			description: "-r with --argocd should render ArgoCD manifests for specific service",
			arguments:   args("render --argocd -e dev -a leonardo"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=argocd,release=leonardo template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
				return nil
			},
		},
		{
			description: "-r with --app-version should set app version",
			arguments:   args("render -e dev -r leonardo --app-version 1.2.3"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.appVersion=1.2.3 template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
				return nil
			},
		},
		{
			description: "-r with --chart-dir should set chart dir and not include --skip-deps",
			argumentsFn: func(ts *TestState) ([]string, error) {
				return args("render -e dev -r leonardo --chart-dir=%s", ts.mockChartSrcDir), nil
			},
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				envPair := fmt.Sprintf("%s=%s", helmfile.ChartPathEnvVar, ts.mockChartSrcDir)
				ts.expectHelmfileCmdWithEnv(devEnv, []string{envPair}, "--log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.chartVersion=local template --output-dir=%s/output/dev", ts.mockHome)
				return nil
			},
		},
		{
			description: "-r with --app-version and --chart-version should set both",
			arguments:   args("render -e dev -r leonardo --app-version 1.2.3 --chart-version 4.5.6"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.appVersion=1.2.3,releases.leonardo.chartVersion=4.5.6 template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
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
				return args("render -e dev -r leonardo --values-file %s", valuesFile), nil
			},
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo template --skip-deps --values=%s/v.yaml --output-dir=%s/output/dev", ts.scratchDir, ts.mockHome)
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
				return args("render -e dev -r leonardo --values-file %s --values-file %s --values-file %s", valuesFiles[0], valuesFiles[1], valuesFiles[2]), nil
			},
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release,release=leonardo template --skip-deps --values=%s/v1.yaml,%s/v2.yaml,%s/v3.yaml --output-dir=%s/output/dev", ts.scratchDir, ts.scratchDir, ts.scratchDir, ts.mockHome)
				return nil
			},
		},
		{
			description: "should fail if repo update fails",
			arguments:   args("render"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd().Return(errors.New("dieee"))
				return nil
			},
			expectedError: regexp.MustCompile("dieee"),
		},
		{
			description: "should fail if helmfile template fails",
			arguments:   args("render -e dev"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(devEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockHome).Return(errors.New("dieee"))
				return nil
			},
			expectedError: regexp.MustCompile("dieee"),
		},
		{
			description: "should run helmfile with --log-level=debug if run with loglevel=debug",
			arguments:   args("render -e dev"),
			setupMocks: func(ts *TestState) error {
				ts.thelmaCLI.setLogLevel("debug")
				ts.expectCmd("helmfile --log-level=debug --allow-no-matching-release repos")
				ts.expectHelmfileCmd(devEnv, "--log-level=debug --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockHome)
				return nil
			},
		},
		{
			description: "--stdout should not render to output directory",
			arguments:   args("render --env=alpha --stdout"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(alphaEnv, "--log-level=info --selector=mode=release template --skip-deps")
				return nil
			},
		},
		{
			description: "-d should render to custom output directory",
			arguments:   args("render -e jdoe -d path/to/nowhere"),
			setupMocks: func(ts *TestState) error {
				ts.expectHelmfileUpdateCmd()
				ts.expectHelmfileCmd(jdoeEnv, "--log-level=info --selector=mode=release template --skip-deps --output-dir=%s/path/to/nowhere/jdoe", Cwd())
				return nil
			},
		},
		{
			description: "missing config directory should raise an error",
			arguments:   args("render"),
			setupMocks: func(ts *TestState) error {
				return os.RemoveAll(ts.mockHome)
			},
			expectedError: regexp.MustCompile("terra-helmfile clone does not exist"),
		},
		{
			description: "missing environments directory should raise an error",
			arguments:   args("render"),
			setupMocks: func(ts *TestState) error {
				return os.RemoveAll(path.Join(ts.mockHome, "environments"))
			},
			expectedError: regexp.MustCompile("environment config directory does not exist"),
		},
		{
			description: "missing clusters directory should raise an error",
			arguments:   args("render"),
			setupMocks: func(ts *TestState) error {
				return os.RemoveAll(path.Join(ts.mockHome, "clusters"))
			},
			expectedError: regexp.MustCompile("cluster config directory does not exist"),
		},
		{
			description: "no environment definitions should raise an error",
			arguments:   args("render"),
			setupMocks: func(ts *TestState) error {
				envDir := path.Join(ts.mockHome, "environments")
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
			ts.thelmaCLI.setArgs(cliArgs)
			err = ts.thelmaCLI.execute()

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
// Eg. args("-e %s", "dev") -> []string{"-e", "dev"}
func args(format string, a ...interface{}) []string {
	formatted := fmt.Sprintf(format, a...)
	return strings.Fields(formatted)
}

// Convenience function for setting up an expectation for a helmfile update command
func (ts *TestState) expectHelmfileUpdateCmd() *shell.Call {
	return ts.expectCmd("helmfile --log-level=info --allow-no-matching-release repos")
}

// Convenience function for setting up an expectation for a helmfile template command
func (ts *TestState) expectHelmfileCmd(target gitops.Target, format string, a ...interface{}) *shell.Call {
	cmd := ts.buildHelmfileCmd(target, format, a...)
	return ts.mockRunner.ExpectCmd(cmd)
}

// Convenience function for setting up an expectation for a helmfile template command
func (ts *TestState) expectHelmfileCmdWithEnv(target gitops.Target, env []string, format string, a ...interface{}) *shell.Call {
	cmd := ts.buildHelmfileCmd(target, format, a...)
	cmd.Env = append(cmd.Env, env...)
	return ts.mockRunner.ExpectCmd(cmd)
}

// Given a release target, and CLI arguments to `helmfile` in the form of a format string and arguments,
// return a matching shell.Command
func (ts *TestState) buildHelmfileCmd(target gitops.Target, format string, a ...interface{}) shell.Command {
	return shell.Command{
		Prog: helmfile.ProgName,
		Args: Args(format, a...),
		Env: []string{
			fmt.Sprintf("%s=%s", helmfile.TargetTypeEnvVar, target.Type()),
			fmt.Sprintf("%s=%s", helmfile.TargetBaseEnvVar, target.Base()),
			fmt.Sprintf("%s=%s", helmfile.TargetNameEnvVar, target.Name()),
		},
		Dir: ts.mockHome,
	}
}

func (ts *TestState) expectCmd(format string, a ...interface{}) *shell.Call {
	cmd := ts.buildCmd(format, a...)
	return ts.mockRunner.ExpectCmd(cmd)
}

func (ts *TestState) buildCmd(format string, a ...interface{}) shell.Command {
	cmd := shell.CmdFromFmt(format, a...)
	cmd.Dir = ts.mockHome
	return cmd
}

// Per-test setup, run before each TestRenderIntegration test case
func setup(t *testing.T) (*TestState, error) {
	var err error

	tmpDir := t.TempDir()
	mockHome := path.Join(t.TempDir(), configRepoName)
	err = os.MkdirAll(mockHome, 0755)
	if err != nil {
		return nil, err
	}

	// Create mock chart dir inside tmp dir
	mockChartSrcDir := path.Join(tmpDir, "charts")
	err = os.MkdirAll(mockChartSrcDir, 0755)
	if err != nil {
		return nil, err
	}

	// Create mock environment and cluster target files
	if err := createFakeTargetFiles(mockHome, fakeReleaseTargets); err != nil {
		return nil, err
	}

	// Create scratch directory, cleaned after every test case.
	scratchDir := path.Join(tmpDir, "scratch")

	// Create a mock runner for executing shell commands
	mockRunnerOpts := shell.MockOptions{
		IgnoreEnvVars: []string{"THF_CHART_CACHE_DIR"}, // ignore chart cache dir (created randomly at runtime)
		VerifyOrder:   false,                           // disable order verification because commands run in parallel
	}
	mockRunner := shell.NewMockRunner(mockRunnerOpts)
	mockRunner.Test(t)

	thelmaCLI := newThelmaCLI()
	thelmaCLI.setHome(mockHome)
	thelmaCLI.setShellRunner(mockRunner)

	ts := &TestState{
		mockHome:        mockHome,
		mockRunner:      mockRunner,
		mockChartSrcDir: mockChartSrcDir,
		scratchDir:      scratchDir,
		thelmaCLI:       thelmaCLI,
	}

	return ts, nil
}

// Create fake target files like `environments/live/alpha.yaml` and `clusters/terra/terra-dev.yaml` in mock config dir
func createFakeTargetFiles(mockConfigRepoPath string, targets []gitops.Target) error {
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
