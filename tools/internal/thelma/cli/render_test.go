package cli

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/helmfile"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/resolver"
	. "github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/testutils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"regexp"
	"testing"
)

// TestRenderArgParsing Given given a set of CLI args, verify that options structures are populated correctly
func TestRenderArgParsing(t *testing.T) {
	type expectedAttrs struct {
		renderOptions *render.Options
		helmfileArgs  *helmfile.Args
	}
	type testConfig struct {
		t         *testing.T
		thelmaCLI *ThelmaCLI
		expected  *expectedAttrs
	}

	testCases := []struct {
		description   string                     // testcase description
		arguments     []string                   // renderCLI args to pass in
		setupFn       func(tc *testConfig) error // optional hook for extra setup
		expectedError *regexp.Regexp             // expected error
	}{
		{
			description:   "invalid argument",
			arguments:     Args("render --foo"),
			expectedError: regexp.MustCompile("unknown flag"),
		},
		{
			description:   "unexpected positional argument",
			arguments:     Args("render foo"),
			expectedError: regexp.MustCompile(`expected no positional arguments, got \[foo]`),
		},
		{
			description:   "-a and -r cannot be combined",
			arguments:     Args("render -r leonardo -a cromwell"),
			expectedError: regexp.MustCompile("one or the other but not both"),
		},
		{
			description:   "-e and -c incompatible",
			arguments:     Args("render -c terra-perf -e dev"),
			expectedError: regexp.MustCompile("only one of --env or --cluster may be specified"),
		},
		{
			description:   "--app-version should require -r",
			arguments:     Args("render --app-version 1.0.0"),
			expectedError: regexp.MustCompile("--app-version requires a release be specified with --release"),
		},
		{
			description:   "--chart-version should require -r",
			arguments:     Args("render --chart-version 1.0.0"),
			expectedError: regexp.MustCompile("--chart-version requires a release be specified with --release"),
		},
		{
			description:   "--chart-dir should require -r",
			arguments:     Args("render --chart-dir %s", t.TempDir()),
			expectedError: regexp.MustCompile("--chart-dir requires a release be specified with --release"),
		},
		{
			description:   "--values-file should require -r",
			arguments:     Args("render --values-file %s", path.Join(t.TempDir(), "does-not-exist.yaml")),
			expectedError: regexp.MustCompile("--values-file requires a release be specified with --release"),
		},
		{
			description:   "--values-file must exist",
			arguments:     Args("render -e dev -r leonardo --values-file %s", path.Join(t.TempDir(), "does-not-exist.yaml")),
			expectedError: regexp.MustCompile("values file does not exist: .*/does-not-exist.yaml"),
		},
		{
			description:   "--chart-dir and --chart-version incompatible",
			arguments:     Args("render -e dev -r leonardo --chart-dir %s --chart-version 1.0.0", t.TempDir()),
			expectedError: regexp.MustCompile("only one of --chart-dir or --chart-version may be specified"),
		},
		{
			description:   "--chart-dir must exist",
			arguments:     Args("render -e dev -r leonardo --chart-dir chart/dir/does/not/exist"),
			expectedError: regexp.MustCompile("chart dir does not exist: .*chart/dir/does/not/exist"),
		},
		{
			description:   "--argocd and --app-version incompatible",
			arguments:     Args("render -e dev -r leonardo --app-version 1.0.0 --argocd"),
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--app-version"),
		},
		{
			description:   "--argocd and --chart-version incompatible",
			arguments:     Args("render -e dev -r leonardo --chart-version 1.0.0 --argocd"),
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--chart-version"),
		},
		{
			description:   "--argocd and --chart-dir incompatible",
			arguments:     Args("render -e dev -r leonardo --chart-dir=%s --argocd", t.TempDir()),
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--chart-dir"),
		},
		{
			description:   "--argocd and --values-file incompatible",
			arguments:     Args("render -e dev -r leonardo --values-file=%s --argocd", "does-not-exist.yaml"),
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--values-file"),
		},
		{
			description:   "--stdout and --output-dir incompatible",
			arguments:     Args("render -e dev -r leonardo -d /tmp/output --stdout"),
			expectedError: regexp.MustCompile("--stdout cannot be used with --output-dir"),
		},
		{
			description:   "--parallel-workers and --stdout incompatible",
			arguments:     Args("render --parallel-workers 10 --stdout"),
			expectedError: regexp.MustCompile("--parallel-workers cannot be used with --stdout"),
		},
		{
			description:   "--cluster and --app-version incompatible",
			arguments:     Args("render --cluster terra-perf -r leonardo --app-version=0.0.1"),
			expectedError: regexp.MustCompile("--app-version cannot be used for cluster releases"),
		},
		{
			description: "config repo path must exist",
			arguments:   []string{"render"},
			setupFn: func(tc *testConfig) error {
				tc.thelmaCLI.setHome("does-not-exist")
				return nil
			},
			expectedError: regexp.MustCompile("terra-helmfile clone does not exist: .*/does-not-exist"),
		},
		{
			description: "config repo path env var must be set",
			arguments:   []string{"render"},
			setupFn: func(tc *testConfig) error {
				tc.thelmaCLI.setHome("")
				return nil
			},
			expectedError: regexp.MustCompile("please specify path to terra-helmfile clone"),
		},
		{
			description:   "no arguments",
			arguments:     []string{"render"},
			expectedError: nil,
		},
		{
			description: "-e should set environment",
			setupFn: func(tc *testConfig) error {
				env := "myenv"
				tc.thelmaCLI.setArgs(Args("render -e %s", env))
				tc.expected.renderOptions.Env = &env
				return nil
			},
		},
		{
			description: "-c should set cluster",
			setupFn: func(tc *testConfig) error {
				cluster := "mycluster"
				tc.thelmaCLI.setArgs(Args("render -c %s", cluster))
				tc.expected.renderOptions.Cluster = &cluster
				return nil
			},
		},
		{
			description: "-d should set output directory",
			setupFn: func(tc *testConfig) error {
				dir := tc.t.TempDir()
				tc.thelmaCLI.setArgs(Args("render -d %s", dir))
				tc.expected.renderOptions.OutputDir = dir
				return nil
			},
		},
		{
			description: "--stdout should set stdout",
			arguments:   Args("render --stdout"),
			setupFn: func(tc *testConfig) error {
				tc.expected.renderOptions.Stdout = true
				return nil
			},
		},
		{
			description: "--parallel-workers should set workers",
			arguments:   Args("render --parallel-workers 32"),
			setupFn: func(tc *testConfig) error {
				tc.expected.renderOptions.ParallelWorkers = 32
				return nil
			},
		},
		{
			description: "--release should set release name",
			arguments:   Args("render --release leonardo"),
			setupFn: func(tc *testConfig) error {
				release := "leonardo"
				tc.expected.renderOptions.Release = &release
				return nil
			},
		},
		{
			description: "--release with env should set release name",
			arguments:   Args("render -e dev --release leonardo"),
			setupFn: func(tc *testConfig) error {
				env, release := "dev", "leonardo"
				tc.expected.renderOptions.Env = &env
				tc.expected.renderOptions.Release = &release
				return nil
			},
		},
		{
			description: "--app-version should set app version",
			arguments:   Args("render -e dev -r leonardo --app-version 1.2.3"),
			setupFn: func(tc *testConfig) error {
				env, release, version := "dev", "leonardo", "1.2.3"
				tc.expected.renderOptions.Env = &env
				tc.expected.renderOptions.Release = &release
				tc.expected.helmfileArgs.AppVersion = &version
				return nil
			},
		},
		{
			description: "--chart-version should set chart version",
			arguments:   Args("render -e dev -r leonardo --chart-version 4.5.6"),
			setupFn: func(tc *testConfig) error {
				env, release, version := "dev", "leonardo", "4.5.6"
				tc.expected.renderOptions.Env = &env
				tc.expected.renderOptions.Release = &release
				tc.expected.helmfileArgs.ChartVersion = &version
				return nil
			},
		},
		{
			description: "--chart-dir should set chart source dir",
			setupFn: func(tc *testConfig) error {
				chartDir := tc.t.TempDir()
				env, release := "dev", "leonardo"
				tc.expected.renderOptions.Env = &env
				tc.expected.renderOptions.Release = &release
				tc.expected.renderOptions.ChartSourceDir = chartDir
				tc.thelmaCLI.setArgs(Args("render -e dev -r leonardo --chart-dir %s", chartDir))
				return nil
			},
		},
		{
			description: "--mode=development should set mode to development",
			setupFn: func(tc *testConfig) error {
				tc.expected.renderOptions.ResolverMode = resolver.Development
				tc.thelmaCLI.setArgs(Args("render --mode development"))
				return nil
			},
		},
		{
			description: "--mode=deploy should set mode to deploy",
			setupFn: func(tc *testConfig) error {
				tc.expected.renderOptions.ResolverMode = resolver.Deploy
				tc.thelmaCLI.setArgs(Args("render --mode deploy"))
				return nil
			},
		},
		{
			description: "--values-file once should set single values file",
			setupFn: func(tc *testConfig) error {
				env, release := "dev", "leonardo"

				valuesDir := tc.t.TempDir()
				valuesFile := path.Join(valuesDir, "v1.yaml")
				if err := os.WriteFile(valuesFile, []byte("# fake values file"), 0644); err != nil {
					return err
				}

				tc.expected.renderOptions.Env = &env
				tc.expected.renderOptions.Release = &release
				tc.expected.helmfileArgs.ValuesFiles = []string{valuesFile}

				tc.thelmaCLI.setArgs(Args("render -e dev -r leonardo --values-file %s", valuesFile))

				return nil
			},
		},
		{
			description: "--values-file multiple times should set multiple values files",
			setupFn: func(tc *testConfig) error {
				env, release := "dev", "leonardo"

				valuesDir := tc.t.TempDir()
				valuesFiles := []string{
					path.Join(valuesDir, "v1.yaml"),
					path.Join(valuesDir, "v2.yaml"),
					path.Join(valuesDir, "v3.yaml"),
				}
				for _, f := range valuesFiles {
					if err := os.WriteFile(f, []byte("# fake values file"), 0644); err != nil {
						return err
					}
				}

				tc.expected.renderOptions.Env = &env
				tc.expected.renderOptions.Release = &release
				tc.expected.helmfileArgs.ValuesFiles = valuesFiles

				tc.thelmaCLI.setArgs(Args("render -e dev -r leonardo --values-file %s --values-file %s --values-file %s", valuesFiles[0], valuesFiles[1], valuesFiles[2]))

				return nil
			},
		},
		{
			description: "--argocd should enable argocd mode",
			arguments:   Args("render --argocd"),
			setupFn: func(tc *testConfig) error {
				tc.expected.helmfileArgs.ArgocdMode = true
				return nil
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			thelmaCLI := newThelmaCLI()

			// Replace render's RunE with a noop function,
			// since we don't want to actually run any renders during this test.
			thelmaCLI.renderCLI.cobraCommand.RunE = func(cmd *cobra.Command, args []string) error {
				return nil
			}

			expected := &expectedAttrs{
				renderOptions: &render.Options{},
				helmfileArgs:  &helmfile.Args{},
			}

			thelmaHome := t.TempDir()
			// set config repo path to a tmp dir
			thelmaCLI.setHome(thelmaHome)

			// add path to expectedAttrs objects so that equals() comparisons succeed
			expected.renderOptions.OutputDir = path.Join(thelmaHome, "output")
			expected.renderOptions.ChartSourceDir = path.Join(thelmaHome, "charts")

			// set other defaults
			expected.renderOptions.ParallelWorkers = 1

			// set command-line args
			thelmaCLI.setArgs(testCase.arguments)

			tc := &testConfig{
				t:         t,
				thelmaCLI: thelmaCLI,
				expected:  expected,
			}

			// call setupFn if defined
			if testCase.setupFn != nil {
				if err := testCase.setupFn(tc); err != nil {
					t.Errorf("setup function returned an error: %v", err)
					return
				}
			}

			// execute the test parsing code
			err := thelmaCLI.execute()

			// if error was expected, check it
			if testCase.expectedError != nil {
				if !assert.Error(t, err, "Expected error matching %v", testCase.expectedError) {
					return
				}
				assert.Regexp(t, testCase.expectedError, err.Error())
				return
			}

			// make sure no error was returned
			if !assert.NoError(t, err, fmt.Errorf("renderCLI.execute() returned unexpected error: %v", err)) {
				return
			}

			// else use default verification
			assert.Equal(t, expected.renderOptions, thelmaCLI.renderCLI.renderOptions)
			assert.Equal(t, expected.helmfileArgs, thelmaCLI.renderCLI.helmfileArgs)
		})
	}
}
