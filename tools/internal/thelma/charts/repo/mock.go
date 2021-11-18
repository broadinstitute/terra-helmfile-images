package repo

import "github.com/stretchr/testify/mock"

type MockRepo struct {
	mock.Mock
}

func NewMockRepo() *MockRepo {
	return &MockRepo{}
}

func (m *MockRepo) RepoURL() string {
	return m.Mock.Called().String(0)
}

func (m *MockRepo) IsLocked() bool {
	return m.Mock.Called().Bool(0)
}

func (m *MockRepo) Unlock() error {
	return m.Mock.Called().Error(0)
}

func (m *MockRepo) Lock() error {
	return m.Mock.Called().Error(0)
}

func (m *MockRepo) UploadChart(fromPath string) error {
	return m.Mock.Called(fromPath).Error(0)
}

func (m *MockRepo) UploadIndex(fromPath string) error {
	return m.Mock.Called(fromPath).Error(0)
}

func (m *MockRepo) HasIndex() (bool, error) {
	result := m.Mock.Called()
	return result.Bool(0), result.Error(1)
}

func (m *MockRepo) DownloadIndex(destPath string) error {
	return m.Mock.Called(destPath).Error(0)
}
