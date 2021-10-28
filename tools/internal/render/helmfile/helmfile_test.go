package helmfile

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock"
	. "github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock/matchers"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

type testState struct {
	configRepo *ConfigRepo
	mockRunner *shellmock.MockRunner
}

func TestHelmfileUpdate(t *testing.T) {
	testCases := []struct{
		description string
		setupMocks func (t *testing.T, ts *testState)
	}{
		{
			description: "info level logging",
			setupMocks: func(t *testing.T, ts *testState) {
				ts.configRepo.helmfileLogLevel = "info"
				ts.mockRunner.ExpectCmd(CmdFromString("helmfile --log-level=info --allow-no-matching-release repos"))
			},
		},
		{
			description: "debug level logging",
			setupMocks: func(t *testing.T, ts *testState) {
				ts.configRepo.helmfileLogLevel = "debug"
				ts.mockRunner.ExpectCmd(CmdFromString("helmfile --log-level=debug --allow-no-matching-release repos"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			ts := setupTestState(t)
			testCase.setupMocks(t, ts)

			err := ts.configRepo.HelmUpdate()
			assert.NoError(t, err)

			ts.mockRunner.AssertExpectations(t)
		})
	}
}

func TestRender(t *testing.T) {
	testCases := []struct{
		description string
		setupMocks func (t *testing.T, ts *testState)
	}{
		{
			description: "info level logging",
			setupMocks: func(t *testing.T, ts *testState) {
				ts.configRepo.helmfileLogLevel = "info"
				ts.mockRunner.ExpectCmd(CmdFromString("helmfile --log-level=info --allow-no-matching-release repos"))
			},
		},
		{
			description: "debug level logging",
			setupMocks: func(t *testing.T, ts *testState) {
				ts.configRepo.helmfileLogLevel = "debug"
				ts.mockRunner.ExpectCmd(CmdFromString("helmfile --log-level=debug --allow-no-matching-release repos"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			ts := setupTestState(t)
			testCase.setupMocks(t, ts)

			err := ts.configRepo.HelmUpdate()
			assert.NoError(t, err)

			ts.mockRunner.AssertExpectations(t)
		})
	}
}

func TestNormalizeOutputDir(t *testing.T) {
	// Create tmpdir
	outputDir := t.TempDir()

	// Create some fake helmfile output directories
	manifestDirs := []string{
		"helmfile-b47efc70-leonardo",
		"helmfile-a14e02c1-cromwell",
		"this-should-not-match",
	}

	for _, manifestDir := range manifestDirs {
		if err := os.MkdirAll(path.Join(outputDir, manifestDir), 0755); err != nil {
			t.Error(err)
			return
		}
	}

	err := normalizeOutputDir(outputDir)
	if !assert.NoError(t, err) {
		return
	}

	for _, dir := range []string{"leonardo", "cromwell", "this-should-not-match"} {
		assert.DirExists(t, path.Join(outputDir, dir))
	}
	assert.NoDirExists(t, path.Join(outputDir, manifestDirs[0]))
	assert.NoDirExists(t, path.Join(outputDir, manifestDirs[1]))
}

func setupTestState(t *testing.T) *testState {
	mockRunner := shellmock.DefaultMockRunner()
	mockRunner.Test(t)

	configRepo := NewConfigRepo(Options{
		Path: t.TempDir(),
		ChartCacheDir: t.TempDir(),
		HelmfileLogLevel: "info",
		ShellRunner: mockRunner,
	})

	return &testState{
		mockRunner: mockRunner,
		configRepo: configRepo,
	}
}
