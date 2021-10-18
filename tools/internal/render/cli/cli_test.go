package cli

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/render"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/render/helmfile"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"regexp"
	"testing"
)

// Given a set of CLI args, verify that options structures are populated correctly
func TestArgumentParsing(t *testing.T) {
	type expectedAttrs struct{
		renderOptions *render.Options
		helmfileArgs *helmfile.Args
	}
	type testConfig struct {
		t *testing.T
		cli *CLI
		expected *expectedAttrs
	}

	testCases := []struct{
		description   string // testcase description
		arguments     []string // cli args to pass in
		setupFn       func (tc *testConfig) error // optional hook for extra setup
		expectedError *regexp.Regexp // expected error
		verifyFn      func (t *testing.T, cli *CLI)
	}{
		{
			description:   "invalid argument",
			arguments:     args("--foo"),
			expectedError: regexp.MustCompile("unknown flag"),
		},
		{
			description:   "unexpected positional argument",
			arguments:     args("foo"),
			expectedError: regexp.MustCompile(`expected no positional arguments, got \[foo]`),
		},
		{
			description:   "-a and -r cannot be combined",
			arguments:     args("-r leonardo -a cromwell"),
			expectedError: regexp.MustCompile("one or the other but not both"),
		},
		{
			description:   "-a should require -e or -c",
			arguments:     args("-a foo"),
			expectedError: regexp.MustCompile(`an environment \(--env\) or cluster \(--cluster\) must be specified when a release is specified with --release`),
		},
		{
			description:   "-r should require -e or -c",
			arguments:     args("-r foo"),
			expectedError: regexp.MustCompile(`an environment \(--env\) or cluster \(--cluster\) must be specified when a release is specified with --release`),
		},
		{
			description:   "-e and -c incompatible",
			arguments:     args("-c terra-perf -e dev"),
			expectedError: regexp.MustCompile("only one of --env or --cluster may be specified"),
		},
		{
			description:   "--app-version should require -r",
			arguments:     args("--app-version 1.0.0"),
			expectedError: regexp.MustCompile("--app-version requires a release be specified with --release"),
		},
		{
			description:   "--chart-version should require -r",
			arguments:     args("--chart-version 1.0.0"),
			expectedError: regexp.MustCompile("--chart-version requires a release be specified with --release"),
		},
		{
			description: "--chart-dir should require -r",
			setupFn: func(tc *testConfig) error {
				tc.cli.setArgs(args("--chart-dir %s", t.TempDir()))
				return nil
			},
			expectedError: regexp.MustCompile("--chart-dir requires a release be specified with --release"),
		},
		{
			description: "--values-file should require -r",
			setupFn: func(tc *testConfig) error {
				tc.cli.setArgs(args("--values-file %s", path.Join(t.TempDir(), "does-not-exist.yaml")))
				return nil
			},
			expectedError: regexp.MustCompile("--values-file requires a release be specified with --release"),
		},
		{
			description: "--values-file must exist",
			setupFn: func(tc *testConfig) error {
				tc.cli.setArgs(args("-e dev -r leonardo --values-file %s", path.Join(t.TempDir(), "does-not-exist.yaml")))
				return nil
			},
			expectedError: regexp.MustCompile("values file does not exist: .*/does-not-exist.yaml"),
		},
		{
			description: "--chart-dir and --chart-version incompatible",
			setupFn: func(tc *testConfig) error {
				tc.cli.setArgs(args("-e dev -r leonardo --chart-dir %s --chart-version 1.0.0", t.TempDir()))
				return nil
			},
			expectedError: regexp.MustCompile("only one of --chart-dir or --chart-version may be specified"),
		},
		{
			description:   "--chart-dir must exist",
			arguments:     args("-e dev -r leonardo --chart-dir chart/dir/does/not/exist"),
			expectedError: regexp.MustCompile("chart dir does not exist: .*chart/dir/does/not/exist"),
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
			description: "--argocd and --chart-dir incompatible",
			setupFn: func(tc *testConfig) error {
				tc.cli.setArgs(args("-e dev -r leonardo --chart-dir=%s --argocd", t.TempDir()))
				return nil
			},
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--chart-dir"),
		},
		{
			description:   "--argocd and --values-file incompatible",
			arguments:     args("-e dev -r leonardo --values-file=%s --argocd", "does-not-exist.yaml"),
			expectedError: regexp.MustCompile("--argocd cannot be used with.*--values-file"),
		},
		{
			description:   "--stdout and --output-dir incompatible",
			arguments:     args("-e dev -r leonardo -d /tmp/output --stdout"),
			expectedError: regexp.MustCompile("--stdout cannot be used with --output-dir"),
		},
		{
			description:   "--parallel-workers and --stdout incompatible",
			arguments:     args("--parallel-workers 10 --stdout"),
			expectedError: regexp.MustCompile("--parallel-workers cannot be used with --stdout"),
		},
		{
			description:   "--cluster and --app-version incompatible",
			arguments:     args("--cluster terra-perf -r leonardo --app-version=0.0.1"),
			expectedError: regexp.MustCompile("--app-version cannot be used for cluster releases"),
		},
		{
			description:   "--scratch-dir must exist",
			arguments:     args("--scratch-dir scratch/dir/does/not/exist"),
			expectedError: regexp.MustCompile("scratch dir does not exist: .*scratch/dir/does/not/exist"),
		},
		{
			description: "config repo path must exist",
			arguments: []string{},
			setupFn: func(tc *testConfig) error {
				tc.cli.setEnvVar(configRepoPathEnvVar, path.Join(t.TempDir(), "does-not-exist"))
				return nil
			},
			expectedError: regexp.MustCompile("config repo clone does not exist: .*/does-not-exist"),
		},
		{
			description: "config repo path env var must be set",
			arguments: []string{},
			setupFn: func(tc *testConfig) error {
				tc.cli.unsetEnvVar(configRepoPathEnvVar)
				return nil
			},
			expectedError: regexp.MustCompile("please specify path to terra-helmfile clone"),
		},
		{
			description: "no arguments",
			expectedError: nil,
		},
		{
			description: "-e should set environment",
			setupFn: func(tc *testConfig) error {
				env := "myenv"
				tc.cli.setArgs(args("-e %s", env))
				tc.expected.renderOptions.Env = &env
				return nil
			},
		},
		{
			description: "-c should set cluster",
			setupFn: func(tc *testConfig) error {
				cluster := "mycluster"
				tc.cli.setArgs(args("-c %s", cluster))
				tc.expected.renderOptions.Cluster = &cluster
				return nil
			},
		},
		{
			description: "-d should set output directory",
			setupFn: func(tc *testConfig) error {
				dir := tc.t.TempDir()
				tc.cli.setArgs(args("-d %s", dir))
				tc.expected.renderOptions.OutputDir = dir
				return nil
			},
		},
		{
			description: "--scratch-dir should set scratch directory",
			setupFn: func(tc *testConfig) error {
				dir := tc.t.TempDir()
				tc.cli.setArgs(args("--scratch-dir %s", dir))
				tc.expected.renderOptions.ScratchDir = &dir
				return nil
			},
		},
		{
			description: "--stdout should set stdout",
			arguments: args("--stdout"),
			setupFn: func(tc *testConfig) error {
				tc.expected.renderOptions.Stdout = true
				return nil
			},
		},
		{
			description: "-v should set verbosity to 1",
			arguments: args("-v"),
			setupFn: func(tc *testConfig) error {
				tc.expected.renderOptions.Verbosity = 1
				return nil
			},
		},
		{
			description: "-v -v should set verbosity to 2",
			arguments: args("-v -v"),
			setupFn: func(tc *testConfig) error {
				tc.expected.renderOptions.Verbosity = 2
				return nil
			},
		},
		{
			description: "--parallel-workers should set workers",
			arguments: args("--parallel-workers 32"),
			setupFn: func(tc *testConfig) error {
				tc.expected.renderOptions.ParallelWorkers = 32
				return nil
			},
		},
		{
			description: "--release should set release name",
			arguments: args("-e dev --release leonardo"),
			setupFn: func(tc *testConfig) error {
				env, release := "dev", "leonardo"
				tc.expected.renderOptions.Env = &env
				tc.expected.helmfileArgs.ReleaseName = &release
				return nil
			},
		},
		{
			description: "--app-version should set app version",
			arguments: args("-e dev -r leonardo --app-version 1.2.3"),
			setupFn: func(tc *testConfig) error {
				env, release, version := "dev", "leonardo", "1.2.3"
				tc.expected.renderOptions.Env = &env
				tc.expected.helmfileArgs.ReleaseName = &release
				tc.expected.helmfileArgs.AppVersion = &version
				return nil
			},
		},
		{
			description: "--chart-version should set chart version",
			arguments: args("-e dev -r leonardo --chart-version 4.5.6"),
			setupFn: func(tc *testConfig) error {
				env, release, version := "dev", "leonardo", "4.5.6"
				tc.expected.renderOptions.Env = &env
				tc.expected.helmfileArgs.ReleaseName = &release
				tc.expected.helmfileArgs.ChartVersion = &version
				return nil
			},
		},
		{
			description: "--chart-dir should set chart dir",
			setupFn: func(tc *testConfig) error {
				chartDir := tc.t.TempDir()
				env, release := "dev", "leonardo"
				tc.expected.renderOptions.Env = &env
				tc.expected.helmfileArgs.ReleaseName = &release
				tc.expected.helmfileArgs.ChartDir = &chartDir
				tc.cli.setArgs(args("-e dev -r leonardo --chart-dir %s", chartDir))
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
				tc.expected.helmfileArgs.ReleaseName = &release
				tc.expected.helmfileArgs.ValuesFiles = []string{valuesFile}

				tc.cli.setArgs(args("-e dev -r leonardo --values-file %s", valuesFile))

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
				tc.expected.helmfileArgs.ReleaseName = &release
				tc.expected.helmfileArgs.ValuesFiles = valuesFiles

				tc.cli.setArgs(args("-e dev -r leonardo --values-file %s --values-file %s --values-file %s", valuesFiles[0], valuesFiles[1], valuesFiles[2]))

				return nil
			},
		},
		{
			description: "--argocd should enable argocd mode",
			arguments: args("--argocd"),
			setupFn: func(tc *testConfig) error {
				tc.expected.helmfileArgs.ArgocdMode = true
				return nil
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			cli := newCLI(true)
			expected := &expectedAttrs{
				renderOptions: &render.Options{},
				helmfileArgs: &helmfile.Args{},
			}

			configRepoPath := t.TempDir()
			// set config repo path to a tmp dir
			cli.setEnvVar(configRepoPathEnvVar, configRepoPath)

			// add path to expectedAttrs objects so that equals() comparisons succeed
			expected.renderOptions.ConfigRepoPath = configRepoPath
			expected.renderOptions.OutputDir = path.Join(configRepoPath, "output")

			// set other defaults
			expected.renderOptions.ParallelWorkers = 1

			// set cli args
			cli.setArgs(testCase.arguments)

			tc := &testConfig{t: t, cli: cli, expected: expected}

			// call setupFn if defined
			if testCase.setupFn != nil {
				if err := testCase.setupFn(tc); err != nil {
					t.Errorf("setup function returned an error: %v", err)
					return
				}
			}

			// execute the test parsing code
			err := cli.execute()

			// if error was expected, check it
			if testCase.expectedError != nil {
				assert.Error(t, err, "Expected error matching %v", testCase.expectedError)
				assert.Regexp(t, testCase.expectedError, err.Error())
				return
			}

			// make sure no error was returned
			if !assert.NoError(t, err, fmt.Errorf("cli.execute() returned unexpected error: %v", err)) {
				return
			}

			// if custom verify function was configured, use it
			if testCase.verifyFn != nil {
				testCase.verifyFn(t, cli)
				return
			}

			// else use default verification
			assert.Equal(t, expected.renderOptions, cli.renderOptions)
			assert.Equal(t, expected.helmfileArgs, cli.helmfileArgs)
		})
	}
}
