package gitops

import (
	"github.com/stretchr/testify/mock"
)

type MockVersions struct {
	mock.Mock
}

func NewMockVersions() *MockVersions {
	return &MockVersions{}
}

func (v *MockVersions) GetSnapshot(releaseType ReleaseType, versionSet VersionSet) VersionSnapshot {
	result := v.Called(releaseType, versionSet)
	return result.Get(0).(VersionSnapshot)
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

func (s *MockSnapshot) AppVersion(releaseName string) string {
	return s.Called(releaseName).String(0)
}

func (s *MockSnapshot) UpdateChartVersionIfDefined(releaseName string, newVersion string) error {
	return s.Called(releaseName, newVersion).Error(0)
}
