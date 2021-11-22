package versions

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"testing"
)

const appDevSnapshotInitialContent = `
releases:
  agora:
    appVersion: ignored # Doesn't matter
    chartVersion: 0.0.0
`


const appDevSnapshotUpdatedContent = `
releases:
  agora:
    appVersion: ignored # Doesn't matter
    chartVersion: 1.2.3
`

const clusterAlphaSnapshotInitialContent = `
releases:
  prometheus:
    chartVersion: 0.1.2
`

const clusterAlphaSnapshotUpdatedContent = `
releases:
  prometheus:
    chartVersion: 4.5.6
`

func TestSnapshot_ChartVersion(t *testing.T) {
	thelmaHome := t.TempDir()
	runner := shellmock.DefaultMockRunner()

	err := initializeFakeVersionsDir(thelmaHome)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	v := NewVersions(thelmaHome, runner)
	s, err := v.LoadSnapshot(AppRelease, Dev)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.True(t, s.ReleaseDefined("agora"))
	assert.Equal(t, "0.0.0", s.ChartVersion("agora"))

	runner.AssertExpectations(t)
}

func TestSnapshot_UpdateChartVersionIfDefined(t *testing.T) {
	type testMocks struct {
		runner *shellmock.MockRunner
		thelmaHome string
	}
	testCases := []struct{
		name string
		releaseName string
		newVersion string
		releaseType ReleaseType
		set Set
		expectedError string
		setupMocks func(testMocks)
	}{
		{
			name: "should set agora version in versions/app/dev.yaml",
			releaseName: "agora",
			newVersion: "1.2.3",
			releaseType: AppRelease,
			set: Dev,
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
					if err := writeFakeAppDevVersionsFile(tm.thelmaHome, appDevSnapshotUpdatedContent); err != nil {
						t.Fatal(err)
					}
				})
			},
		},
		{
			name: "should set prometheus version in versions/cluster/alpha.yaml",
			releaseName: "prometheus",
			newVersion: "4.5.6",
			releaseType: ClusterRelease,
			set: Alpha,
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
					if err := writeFakeClusterAlphaVersionsFile(tm.thelmaHome, clusterAlphaSnapshotUpdatedContent); err != nil {
						t.Fatal(err)
					}
				})
			},
		},
		{
			name: "should NOT version for undefined release",
			releaseName: "fakechart",
			newVersion: "1.2.3",
			releaseType: AppRelease,
			set: Dev,
		},

	}

	for _, tc := range testCases {
		mocks := testMocks{
			thelmaHome: t.TempDir(),
			runner: shellmock.DefaultMockRunner(),
		}
		_versions := NewVersions(mocks.thelmaHome, mocks.runner)

		err := initializeFakeVersionsDir(mocks.thelmaHome)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		if tc.setupMocks != nil {
			tc.setupMocks(mocks)
		}

		_snapshot, err := _versions.LoadSnapshot(tc.releaseType, tc.set)
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
	assert.Equal(t, "app", AppRelease.String())
	assert.Equal(t, "cluster", ClusterRelease.String())
}

func TestReleaseType_UnmarshalYAML(t *testing.T) {
	var err error
	var r ReleaseType

	err = yaml.Unmarshal([]byte("app"), &r)
	assert.NoError(t, err)
	assert.Equal(t, AppRelease, r)

	err = yaml.Unmarshal([]byte("cluster"), &r)
	assert.NoError(t, err)
	assert.Equal(t, ClusterRelease, r)

	err = yaml.Unmarshal([]byte("invalid"), &r)
	assert.Error(t, err)
	assert.Regexp(t, "unknown release type", err)
}

func TestMocks_MatchInterface(t *testing.T) {
	v := NewMockVersions()
	s := NewMockSnapshot()

	// make sure interfaces match -- compilation will fail if they don't
	var _ Versions = v
	var _ Snapshot = s
}

func initializeFakeVersionsDir(thelmaHome string) error {
	if err := writeFakeAppDevVersionsFile(thelmaHome, appDevSnapshotInitialContent); err != nil {
		return err
	}
	if err := writeFakeClusterAlphaVersionsFile(thelmaHome, clusterAlphaSnapshotInitialContent); err != nil {
		return err
	}

	return nil
}

func writeFakeAppDevVersionsFile(thelmaHome string, content string) error {
	file := path.Join(thelmaHome, versionsDir, "app", "dev.yaml")
	return writeFakeVersionsFile(file, content)
}

func writeFakeClusterAlphaVersionsFile(thelmaHome string, content string) error {
	file := path.Join(thelmaHome, versionsDir, "cluster", "alpha.yaml")
	return writeFakeVersionsFile(file, content)
}

func writeFakeVersionsFile(fileName string, content string) error {
	if err := os.MkdirAll(path.Dir(fileName), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
		return err
	}
	return nil
}
