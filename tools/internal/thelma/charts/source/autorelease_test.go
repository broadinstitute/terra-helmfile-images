package source

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/versions"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

func TestAutoReleaser_UpdateVersionsFile(t *testing.T) {
	chartName := "mychart"
	newVersion := "5.6.7"

	testCases := []struct {
		name          string
		newVersion    string
		configContent string
		setupMocks    func(*versions.MockVersions)
		matchErr      string
	}{
		{
			name: "No config file should default to enabled + app release type",
			setupMocks: func(mockVersions *versions.MockVersions) {
				mockVersions.On("SetReleaseVersionIfDefined", chartName, versions.AppRelease, versions.Dev, newVersion).Return(nil)
			},
		},
		{
			name:          "Should not update release version if disabled in config file",
			configContent: `enabled: false`,
		},
		{
			name:          "Should permit release name overriding",
			configContent: `release: {name: foo}`,
			setupMocks: func(mockVersions *versions.MockVersions) {
				mockVersions.On("SetReleaseVersionIfDefined", "foo", versions.AppRelease, versions.Dev, newVersion).Return(nil)
			},
		},
		{
			name:          "Should permit release type overriding",
			configContent: `release: {type: cluster}`,
			setupMocks: func(mockVersions *versions.MockVersions) {
				mockVersions.On("SetReleaseVersionIfDefined", chartName, versions.ClusterRelease, versions.Dev, newVersion).Return(nil)
			},
		},
	}
	for _, tc := range testCases {
		chartDir := t.TempDir()
		chart := NewMockChart()
		chart.On("Name").Return("mychart")
		chart.On("Path").Return(chartDir)

		t.Run(tc.name, func(t *testing.T) {
			mockVersions := versions.NewMockVersions()
			if tc.setupMocks != nil {
				tc.setupMocks(mockVersions)
			}

			if len(tc.configContent) > 0 {
				if err := os.WriteFile(path.Join(chartDir, configFile), []byte(tc.configContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			_autoReleaser := NewAutoReleaser(mockVersions)
			err := _autoReleaser.UpdateVersionsFile(chart, newVersion)

			mockVersions.AssertExpectations(t)

			if len(tc.matchErr) == 0 {
				assert.NoError(t, err)
				return
			}

			assert.Error(t, err)
			assert.Regexp(t, tc.matchErr, err)
		})
	}
}
