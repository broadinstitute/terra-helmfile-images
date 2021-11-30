package versions

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops/release"
	"github.com/stretchr/testify/mock"
)

type MockVersions struct {
	mock.Mock
}

func NewMockVersions() *MockVersions {
	return &MockVersions{}
}

func (v *MockVersions) LoadSnapshot(releaseType release.ReleaseType, versionSet Set) (Snapshot, error) {
	result := v.Called(releaseType, versionSet)
	return result.Get(0).(Snapshot), result.Error(1)
}

type MockSnapshot struct {
	mock.Mock
}

func NewMockSnapshot() *MockSnapshot {
	return &MockSnapshot{}
}

func (s *MockSnapshot) ReleaseDefined(releaseName string) bool {
	return s.Called(releaseName).Bool(0)
}

func (s *MockSnapshot) ChartVersion(releaseName string) string {
	return s.Called(releaseName).String(0)
}

func (s *MockSnapshot) UpdateChartVersionIfDefined(releaseName string, newVersion string) error {
	return s.Called(releaseName, newVersion).Error(0)
}
