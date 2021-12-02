package source

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

func TestAutoReleaser_UpdateVersionsFile(t *testing.T) {
	chartName := "mychart"
	newVersion := "5.6.7"

	type mocks struct {
		versions *gitops.MockVersions
		snapshot *gitops.MockSnapshot
	}

	testCases := []struct {
		name          string
		newVersion    string
		configContent string
		setupMocks    func(mocks)
		matchErr      string
	}{
		{
			name: "No config file should default to enabled + app release type",
			setupMocks: func(m mocks) {
				m.versions.On("GetSnapshot", gitops.AppReleaseType, gitops.Dev).Return(m.snapshot, nil)
				m.snapshot.On("UpdateChartVersionIfDefined", chartName, newVersion).Return(nil)
			},
		},
		{
			name:          "Should not update release version if disabled in config file",
			configContent: `enabled: false`,
		},
		{
			name:          "Should support release name overriding",
			configContent: `release: {name: foo}`,
			setupMocks: func(m mocks) {
				m.versions.On("GetSnapshot", gitops.AppReleaseType, gitops.Dev).Return(m.snapshot, nil)
				m.snapshot.On("UpdateChartVersionIfDefined", "foo", newVersion).Return(nil)
			},
		},
		{
			name:          "Should support release type overriding",
			configContent: `release: {type: cluster}`,
			setupMocks: func(m mocks) {
				m.versions.On("GetSnapshot", gitops.ClusterReleaseType, gitops.Dev).Return(m.snapshot, nil)
				m.snapshot.On("UpdateChartVersionIfDefined", chartName, newVersion).Return(nil)
			},
		},
	}
	for _, tc := range testCases {
		chartDir := t.TempDir()
		chart := NewMockChart()
		chart.On("Name").Return("mychart")
		chart.On("Path").Return(chartDir)

		t.Run(tc.name, func(t *testing.T) {
			m := mocks{
				versions: gitops.NewMockVersions(),
				snapshot: gitops.NewMockSnapshot(),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}

			if len(tc.configContent) > 0 {
				if err := os.WriteFile(path.Join(chartDir, configFile), []byte(tc.configContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			_autoReleaser := NewAutoReleaser(m.versions)
			err := _autoReleaser.UpdateReleaseVersion(chart, newVersion)

			m.versions.AssertExpectations(t)
			m.snapshot.AssertExpectations(t)

			if len(tc.matchErr) == 0 {
				assert.NoError(t, err)
				return
			}

			assert.Error(t, err)
			assert.Regexp(t, tc.matchErr, err)
		})
	}
}
