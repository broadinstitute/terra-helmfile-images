package gitops

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"testing"
)

const updateContent1 = `
releases:
  agora:
    appVersion: v100
    chartVersion: 1.2.3
`

const updateContent2 = `
releases:
  prometheus:
    chartVersion: 4.5.6
`

func TestSnapshot_ChartVersion(t *testing.T) {
	thelmaHome := t.TempDir()
	runner := shell.DefaultMockRunner()

	err := initializeFakeVersionsDir(thelmaHome)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	v, err := NewVersions(thelmaHome, runner)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	s1 := v.GetSnapshot(AppReleaseType, Dev)

	assert.True(t, s1.ReleaseDefined("agora"))
	assert.Equal(t, "0.10.0", s1.ChartVersion("agora"))
	assert.Equal(t, "v100", s1.AppVersion("agora"))

	s2 := v.GetSnapshot(ClusterReleaseType, Alpha)
	assert.True(t, s2.ReleaseDefined("prometheus"))
	assert.Equal(t, "0.1.3", s2.ChartVersion("prometheus"))
	assert.Equal(t, "", s2.AppVersion("prometheus"))

	runner.AssertExpectations(t)
}

func TestSnapshot_UpdateChartVersionIfDefined(t *testing.T) {
	type testMocks struct {
		runner     *shell.MockRunner
		thelmaHome string
	}
	testCases := []struct {
		name          string
		releaseName   string
		newVersion    string
		releaseType   ReleaseType
		set           VersionSet
		expectedError string
		setupMocks    func(testMocks)
	}{
		{
			name:        "should set agora version in versions/app/dev.yaml",
			releaseName: "agora",
			newVersion:  "1.2.3",
			releaseType: AppReleaseType,
			set:         Dev,
			setupMocks: func(tm testMocks) {
				tm.runner.ExpectCmd(shell.Command{
					Prog: "yq",
					Args: []string{
						"eval",
						"--inplace",
						`.releases.agora.chartVersion = "1.2.3"`,
						path.Join(tm.thelmaHome, "versions/app/dev.yaml"),
					},
				}).Run(func(args mock.Arguments) {
					if err := writeFakeVersionsFile(tm.thelmaHome, AppReleaseType, Dev, updateContent1); err != nil {
						t.Fatal(err)
					}
				})
			},
		},
		{
			name:        "should set prometheus version in versions/cluster/alpha.yaml",
			releaseName: "prometheus",
			newVersion:  "4.5.6",
			releaseType: ClusterReleaseType,
			set:         Alpha,
			setupMocks: func(tm testMocks) {
				tm.runner.ExpectCmd(shell.Command{
					Prog: "yq",
					Args: []string{
						"eval",
						"--inplace",
						`.releases.prometheus.chartVersion = "4.5.6"`,
						path.Join(tm.thelmaHome, "versions/cluster/alpha.yaml"),
					},
				}).Run(func(args mock.Arguments) {
					if err := writeFakeVersionsFile(tm.thelmaHome, ClusterReleaseType, Alpha, updateContent2); err != nil {
						t.Fatal(err)
					}
				})
			},
		},
		{
			name:        "should NOT update version for undefined release",
			releaseName: "fakechart",
			newVersion:  "1.2.3",
			releaseType: AppReleaseType,
			set:         Dev,
		},
		{
			name:        "should NOT update version if new version < existing version",
			releaseName: "agora",
			newVersion:  "0.9.0",
			releaseType: AppReleaseType,
			set:         Dev,
		},
		{
			name:        "should NOT update version if new version == existing version",
			releaseName: "agora",
			newVersion:  "0.10.0",
			releaseType: AppReleaseType,
			set:         Dev,
		},
	}

	for _, tc := range testCases {
		mocks := testMocks{
			thelmaHome: t.TempDir(),
			runner:     shell.DefaultMockRunner(),
		}
		err := initializeFakeVersionsDir(mocks.thelmaHome)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		_versions, err := NewVersions(mocks.thelmaHome, mocks.runner)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		if tc.setupMocks != nil {
			tc.setupMocks(mocks)
		}

		_snapshot := _versions.GetSnapshot(tc.releaseType, tc.set)
		assert.NoError(t, err)
		err = _snapshot.UpdateChartVersionIfDefined(tc.releaseName, tc.newVersion)

		if tc.expectedError == "" {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
			assert.Regexp(t, tc.expectedError, err)
		}
	}
}

func TestReleaseType_String(t *testing.T) {
	assert.Equal(t, "app", AppReleaseType.String())
	assert.Equal(t, "cluster", ClusterReleaseType.String())
}

func TestReleaseType_UnmarshalYAML(t *testing.T) {
	var err error
	var r ReleaseType

	err = yaml.Unmarshal([]byte("app"), &r)
	assert.NoError(t, err)
	assert.Equal(t, AppReleaseType, r)

	err = yaml.Unmarshal([]byte("cluster"), &r)
	assert.NoError(t, err)
	assert.Equal(t, ClusterReleaseType, r)

	err = yaml.Unmarshal([]byte("invalid"), &r)
	assert.Error(t, err)
	assert.Regexp(t, "unknown release type", err)
}

func TestMocks_MatchInterface(t *testing.T) {
	v := NewMockVersions()
	s := NewMockSnapshot()

	// make sure interfaces match -- compilation will fail if they don't
	var _ Versions = v
	var _ VersionSnapshot = s
}

func initializeFakeVersionsDir(thelmaHome string) error {
	runner := shell.NewDefaultRunner()
	cmd := shell.Command{
		Prog: "cp",
		Args: []string{"-r", "testdata/versions", path.Join(thelmaHome, "versions")},
	}
	return runner.Run(cmd)
}

func writeFakeVersionsFile(thelmaHome string, releaseType ReleaseType, set VersionSet, content string) error {
	file := path.Join(thelmaHome, "versions", releaseType.String(), fmt.Sprintf("%s%s", set.String(), ".yaml"))
	return os.WriteFile(file, []byte(content), 0666)
}
