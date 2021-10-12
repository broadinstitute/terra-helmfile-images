package render

import (
	"errors"
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

// This file contains an integration test for the render utility

// Fake environments and clusters, mocked for integration mock
var fakeReleaseTargets = []ReleaseTarget{
	NewEnvironment("dev", "live"),
	NewEnvironment("alpha", "live"),
	NewEnvironment("jdoe", "personal"),
	NewCluster("terra-perf", "terra"),
	NewCluster("tdr-staging", "tdr"),
}

// Struct for tracking global state that is mocked when a test executes and restored/cleaned up after
type TestState struct {
	mockRunner *shellmock.MockRunner // mock shell.Runner, used for mocking shell commmands
	mockConfigRepoPath     string // mock terra-helmfile, created once before all test cases
	mockChartDir           string // mock chart directory, created once before all test cases
	scratchDir             string // scratch directory, cleaned out before each test case
	rootDir                string // root/parent directory for all test files
	originalConfigRepoPath string // real config repo path, saved before tests start and restored after they finish
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
		description      string                                            // Testcase description
		arguments        []string                                          // Fake user-supplied CLI arguments to pass to `render`
		argumentsFn      func(ts *TestState) ([]string, error)              // Callback function returning fake user-supplied CLI arguments to pass to `render`. Will override `arguments` field if given
		expectedError    *regexp.Regexp                                    // Optional error we expect to be returned when we execute the Cobra command
		setupMocks       func(m *shellmock.MockRunner, ts *TestState) error // Optional hook mocking expected shell commands
	}{
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
			argumentsFn: func(ts *TestState) ([]string, error) {
				return args("--chart-dir %s", ts.mockChartDir), nil
			},
			expectedError: regexp.MustCompile("--chart-dir requires a release be specified with -r"),
		},
		{
			description:   "--values-file should require -r",
			argumentsFn: func(ts *TestState) ([]string, error) {
				return args("--values-file %s", path.Join(ts.rootDir, "missing.yaml")), nil
			},
			expectedError: regexp.MustCompile("--values-file requires a release be specified with -r"),
		},
		{
			description:   "--values-file must exist",
			argumentsFn: func (ts *TestState) ([]string, error) {
				return args("-e dev -r leonardo --values-file %s", path.Join(ts.rootDir, "missing.yaml")), nil
			},
			expectedError: regexp.MustCompile("values file does not exist: .*/missing.yaml"),
		},
		{
			description:   "--chart-dir and --chart-version incompatible",
			argumentsFn: func (ts *TestState) ([]string, error) {
				return args("-e dev -r leonardo --chart-dir %s --chart-version 1.0.0", ts.mockChartDir), nil
			},
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
			argumentsFn: func(ts *TestState) ([]string, error) {
				return args("-e dev -r leonardo --chart-dir=%s --argocd", ts.mockChartDir), nil
			},
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
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				return nil
			},
			expectedError: regexp.MustCompile("unknown environment: foo"),
		},
		{
			description: "unknown cluster should return error",
			arguments:   args("-c blargh"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				return nil
			},
			expectedError: regexp.MustCompile("unknown cluster: blargh"),
		},
		{
			description: "no arguments should render for all targets",
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=cluster     THF_TARGET_BASE=tdr      THF_TARGET_NAME=tdr-staging helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath)
				ts.cmd("THF_TARGET_TYPE=cluster     THF_TARGET_BASE=terra    THF_TARGET_NAME=terra-perf  helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath)
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live     THF_TARGET_NAME=alpha       helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/alpha", ts.mockConfigRepoPath)
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live     THF_TARGET_NAME=dev         helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=personal THF_TARGET_NAME=jdoe        helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/jdoe", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "--argocd without -e or -a should render Argo manifests for all environments",
			arguments:   args("--argocd"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=cluster     THF_TARGET_BASE=tdr      THF_TARGET_NAME=tdr-staging helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath)
				ts.cmd("THF_TARGET_TYPE=cluster     THF_TARGET_BASE=terra    THF_TARGET_NAME=terra-perf  helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath)
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live     THF_TARGET_NAME=alpha       helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/alpha", ts.mockConfigRepoPath)
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live     THF_TARGET_NAME=dev         helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=personal THF_TARGET_NAME=jdoe        helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/jdoe", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-e should render for specific environment",
			arguments:   args("-e dev"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-c should render for specific cluster",
			arguments:   args("-c terra-perf"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=cluster THF_TARGET_BASE=terra THF_TARGET_NAME=terra-perf helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/terra-perf", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-e with --argocd should render ArgoCD manifests for specific environment",
			arguments:   args("-e dev --argocd"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-c with --argocd should render ArgoCD manifests for specific cluster",
			arguments:   args("-c tdr-staging --argocd"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=cluster THF_TARGET_BASE=tdr THF_TARGET_NAME=tdr-staging helmfile --log-level=info --selector=mode=argocd template --skip-deps --output-dir=%s/output/tdr-staging", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r should render for specific service",
			arguments:   args("-e dev -r leonardo"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with --argocd should render ArgoCD manifests for specific service",
			arguments:   args("--argocd -e dev -a leonardo"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=argocd,release=leonardo template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with --app-version should set app version",
			arguments:   args("-e dev -r leonardo --app-version 1.2.3"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.appVersion=1.2.3 template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with --chart-dir should set chart dir and not include --skip-deps",
			argumentsFn: func(ts *TestState) ([]string, error) {
				return args("-e dev -r leonardo --chart-dir=%s", ts.mockChartDir), nil
			},
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.repo=%s template --output-dir=%s/output/dev", ts.mockChartDir, ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "-r with --app-version and --chart-version should set both",
			arguments:   args("-e dev -r leonardo --app-version 1.2.3 --chart-version 4.5.6"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo --state-values-set=releases.leonardo.appVersion=1.2.3,releases.leonardo.chartVersion=4.5.6 template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
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
				return args(" -e dev -r leonardo --values-file %s", valuesFile), nil
			},
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo template --skip-deps --values=%s/v.yaml --output-dir=%s/output/dev", ts.scratchDir, ts.mockConfigRepoPath)
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
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=release,release=leonardo template --skip-deps --values=%s/v1.yaml,%s/v2.yaml,%s/v3.yaml --output-dir=%s/output/dev", ts.scratchDir, ts.scratchDir, ts.scratchDir, ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "should fail if repo update fails",
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.failCmd("dieee", "helmfile --log-level=info --allow-no-matching-release repos")
				return nil
			},
			expectedError: regexp.MustCompile("dieee"),
		},
		{
			description: "should fail if helmfile template fails",
			arguments:   args("-e dev"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.failCmd("dieee", "THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
			expectedError: regexp.MustCompile("dieee"),
		},
		{
			description: "should run helmfile with --log-level=debug if run with -v -v",
			arguments:   args("-e dev -v -v"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=debug --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=dev helmfile --log-level=debug --selector=mode=release template --skip-deps --output-dir=%s/output/dev", ts.mockConfigRepoPath)
				return nil
			},
		},
		{
			description: "--stdout should not render to output directory",
			arguments:   args("--env=alpha --stdout"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=live THF_TARGET_NAME=alpha helmfile --log-level=info --selector=mode=release template --skip-deps")
				return nil
			},
		},
		{
			description: "-d should render to custom output directory",
			arguments:   args("-e jdoe -d path/to/nowhere"),
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				ts.cmd("helmfile --log-level=info --allow-no-matching-release repos")
				ts.cmd("THF_TARGET_TYPE=environment THF_TARGET_BASE=personal THF_TARGET_NAME=jdoe helmfile --log-level=info --selector=mode=release template --skip-deps --output-dir=%s/path/to/nowhere/jdoe", cwd())
				return nil
			},
		},
		{
			description: "two environments with the same name should raise an error",
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				return createFakeTargetFiles(ts.mockConfigRepoPath, []ReleaseTarget{NewEnvironmentGeneric("dev", "personal")})
			},
			expectedError: regexp.MustCompile(`environment name conflict dev \(personal\) and dev \(live\)`),
		},
		{
			description: "two clusters with the same name should raise an error",
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				return createFakeTargetFiles(ts.mockConfigRepoPath, []ReleaseTarget{NewClusterGeneric("terra-perf", "tdr")})
			},
			expectedError: regexp.MustCompile(`cluster name conflict terra-perf \(terra\) and terra-perf \(tdr\)`),
		},
		{
			description: "environment and cluster with the same name should raise an error",
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				return createFakeTargetFiles(ts.mockConfigRepoPath, []ReleaseTarget{NewClusterGeneric("dev", "terra")})
			},
			expectedError: regexp.MustCompile("cluster name dev conflicts with environment name dev"),
		},
		{
			description: "missing config directory should raise an error",
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				return os.RemoveAll(ts.mockConfigRepoPath)
			},
			expectedError: regexp.MustCompile("does not exist, is it a terra-helmfile clone"),
		},
		{
			description: "missing environments directory should raise an error",
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
				return os.RemoveAll(path.Join(ts.mockConfigRepoPath, "environments"))
			},
			expectedError: regexp.MustCompile("does not exist, is it a terra-helmfile clone"),
		},
		{
			description: "no environment definitions should raise an error",
			setupMocks: func(m *shellmock.MockRunner, ts *TestState) error {
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
			// TODO collapse setup() and setupTestCase() functions
			// Set up mocked state
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

			if err := ts.setupTestCase(); err != nil {
				t.Error(err)
				return
			}

			// Create a mock runner for executing shell commands
			mockRunner := shellmock.DefaultMockRunner()
			mockRunner.Test(t)
			ts.mockRunner = mockRunner

			// Set up expectations for this test case's commands
			if testCase.setupMocks != nil {
				if err := testCase.setupMocks(mockRunner, ts); err != nil {
					t.Errorf("setupMocks error: %v", err)
				}
			}

			// Replace real shell runner with mock runner and then restore when this test completes
			originalRunner := shellRunner
			shellRunner = mockRunner
			defer func() { shellRunner = originalRunner }()

			// Get arguments to pass to Cobra command
			cobraArgs := testCase.arguments
			if testCase.argumentsFn != nil {
				var err error
				cobraArgs, err = testCase.argumentsFn(ts)
				if err != nil {
					t.Errorf("argumentsFn error: %v", err)
				}
			}

			// Run the Cobra command
			cobraCmd := newCobraCommand()
			cobraCmd.SetArgs(cobraArgs)
			err = cobraCmd.Execute()

			// Verify error matches expectations
			if testCase.expectedError == nil {
				assert.Nil(t, err, "Unexpected error returned: %v", err)
			} else {
				if !assert.Error(t, err, "Expected command execution to return an error, but it did not") {
					return
				}
				assert.Regexp(t, testCase.expectedError, err.Error(), "Error mismatch")
			}

			// Verify all expected commands were run
			mockRunner.AssertExpectations(t)
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

	return ts.mockRunner.OnCmd(cmd)
}

// Convenience function to create a failing ExpectedCommand with an error
// a format string _for_ the command.
//
// Eg. cmd("helmfile -e %s template", "alpha")
func (ts *TestState) failCmd(err string, format string, a ...interface{}) *mock.Call {
	call := ts.cmd(format, a...)
	return call.Return(errors.New(err))
}

// Per-test case setup
func (ts *TestState) setupTestCase() error {
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

	// Create mock chart dir inside tmp dir
	mockChartDir := path.Join(tmpDir, "charts")
	err = os.MkdirAll(mockChartDir, 0755)
	if err != nil {
		return nil, err
	}

	// Create scratch directory, cleaned after every test case.
	scratchDir := path.Join(tmpDir, "scratch")

	return &TestState{
		originalConfigRepoPath: originalConfigRepoPath,
		mockConfigRepoPath:     mockConfigRepoPath,
		mockChartDir:           mockChartDir,
		scratchDir:             scratchDir,
		rootDir:                tmpDir,
	}, nil
}

// One-time cleanup, run after all TestCases have run
func cleanup(state *TestState) error {
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
