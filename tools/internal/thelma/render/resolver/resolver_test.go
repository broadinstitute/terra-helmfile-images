package resolver

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"path"
	"testing"
)

const chartSourceDir = "testdata/charts"
const fakeChartName = "fakechart"
const fakeChartVersion = "0.10.0"
const fakeChartRepo = "terra-helm"

type testCfg struct {
	srcDir     string
	cacheDir   string
	scratchDir string
	mockRunner *shell.MockRunner
	input      *ChartRelease
}

func TestResolver(t *testing.T) {
	type expect struct {
		path       string
		version    string
		sourceDesc string
	}

	testCases := []struct {
		name       string
		mode       Mode
		errMatcher string
		setupMocks func(*testCfg)
		expect     func(*expect, *testCfg)
	}{
		{
			name: "development mode should successfully resolve chart from source directory",
			mode: Development,
			setupMocks: func(tc *testCfg) {
				tc.expectHelmDependencyUpdate()
			},
			expect: func(e *expect, _ *testCfg) {
				e.path = path.Join(chartSourceDir, fakeChartName)
				e.version = fakeChartVersion
				e.sourceDesc = fmt.Sprintf("./%s/%s", chartSourceDir, fakeChartName)
			},
		},
		{
			name: "development mode should fall back to remote resolver if chart does not exist in source directory",
			mode: Development,
			setupMocks: func(tc *testCfg) {
				tc.input.Name = "datarepo"
				tc.input.Version = "4.5.6"
				tc.input.Repo = "datarepo-helm"
				tc.expectHelmFetch(true)
			},
			expect: func(e *expect, tc *testCfg) {
				e.path = path.Join(tc.cacheDir, "datarepo-helm", "datarepo-4.5.6")
				e.version = "4.5.6"
				e.sourceDesc = "datarepo-helm"
			},
		},
		{
			name: "development mode should return error if chart does not exist in source directory and fetch fails",
			mode: Development,
			setupMocks: func(tc *testCfg) {
				tc.input.Name = "datarepo"
				tc.input.Version = "4.5.6"
				tc.input.Repo = "datarepo-helm"
				tc.expectHelmFetch(false)
			},
			errMatcher: "error downloading chart",
		},
		{
			name: "deploy mode should download from Helm repo",
			mode: Deploy,
			setupMocks: func(tc *testCfg) {
				tc.expectHelmFetch(true)
			},
			expect: func(e *expect, tc *testCfg) {
				e.path = path.Join(tc.cacheDir, fakeChartRepo, fmt.Sprintf("%s-%s", fakeChartName, fakeChartVersion))
				e.version = fakeChartVersion
				e.sourceDesc = fakeChartRepo
			},
		},
		{
			name: "deploy mode should fall back to source if download fails",
			mode: Deploy,
			setupMocks: func(tc *testCfg) {
				tc.expectHelmFetch(false)
				tc.expectHelmDependencyUpdate()
			},
			expect: func(e *expect, _ *testCfg) {
				e.path = path.Join(chartSourceDir, fakeChartName)
				e.version = fakeChartVersion
				e.sourceDesc = fmt.Sprintf("./%s/%s", chartSourceDir, fakeChartName)
			},
		},
		{
			name: "deploy mode should fail if download fails and source version does not match chart release",
			mode: Deploy,
			setupMocks: func(tc *testCfg) {
				tc.input.Version = "3.2.1"
				tc.expectHelmFetch(false)
			},
			errMatcher: "error downloading chart",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := testCfg{
				srcDir:     chartSourceDir,
				cacheDir:   t.TempDir(),
				scratchDir: t.TempDir(),
				mockRunner: shell.DefaultMockRunner(),
				input: &ChartRelease{
					Repo:    fakeChartRepo,
					Name:    fakeChartName,
					Version: fakeChartVersion,
				},
			}

			if testCase.setupMocks != nil {
				testCase.setupMocks(&cfg)
			}

			expected := expect{
				path:       "TODO - set in test case",
				version:    "TODO - set in test case",
				sourceDesc: "TODO - set in test case",
			}

			if testCase.expect != nil {
				testCase.expect(&expected, &cfg)
			}

			resolver := NewResolver(cfg.mockRunner, Options{
				Mode:       testCase.mode,
				SourceDir:  cfg.srcDir,
				CacheDir:   cfg.cacheDir,
				ScratchDir: cfg.scratchDir,
			})

			result, err := resolver.Resolve(*cfg.input)

			if testCase.errMatcher != "" {
				if !assert.Error(t, err) {
					t.FailNow()
				}
				assert.Regexp(t, testCase.errMatcher, err.Error())
				return
			}

			if !assert.NoError(t, err) {
				t.FailNow()
			}

			assert.Equal(t, expected.path, result.Path())
			assert.Equal(t, expected.sourceDesc, result.SourceDescription())
			assert.Equal(t, expected.version, result.Version())
		})
	}
}

func (tc *testCfg) expectHelmDependencyUpdate() {
	tc.mockRunner.ExpectCmd(shell.Command{
		Prog: "helm",
		Args: []string{
			"dependency",
			"update",
			"--skip-refresh",
		},
		Dir: path.Join(chartSourceDir, tc.input.Name),
	})
}

func (tc *testCfg) expectHelmFetch(success bool) {
	downloadDir := path.Join(
		tc.scratchDir,
		fmt.Sprintf("%s-%s-%s", tc.input.Repo, tc.input.Name, tc.input.Version),
	)

	call := tc.mockRunner.ExpectCmd(shell.Command{
		Prog: "helm",
		Args: []string{
			"fetch",
			fmt.Sprintf("%s/%s", tc.input.Repo, tc.input.Name),
			"--version",
			tc.input.Version,
			"--untar",
			"-d",
			downloadDir,
		},
	})

	if success {
		call.Run(func(args mock.Arguments) {
			fakeChartDir := path.Join(downloadDir, tc.input.Name)
			if err := os.MkdirAll(fakeChartDir, 0775); err != nil {
				panic(fmt.Errorf("failed to create fake fetch dir %s: %v", fakeChartDir, err))
			}
		})
	} else {
		call.WithStderr("helm fetch failed!").ExitsNonZero()
	}
}
