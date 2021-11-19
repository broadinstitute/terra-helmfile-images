package versions

import "github.com/stretchr/testify/mock"

type MockVersions struct {
	mock.Mock
}

func NewMockVersions() *MockVersions {
	return &MockVersions{}
}

func (v *MockVersions) SetReleaseVersionIfDefined(releaseName string, releaseType ReleaseType, versionSet Set, newVersion string) error {
	return v.Called(releaseName, releaseType, versionSet, newVersion).Error(0)
}
