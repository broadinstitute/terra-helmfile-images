package index

import "github.com/stretchr/testify/mock"

type MockIndex struct {
	mock.Mock
}

func NewMockIndex() *MockIndex {
	return &MockIndex{}
}

func (m *MockIndex) Versions(chartName string) []string {
	return m.Called(chartName).Get(0).([]string)
}

func (m *MockIndex) HasVersion(chartName string, version string) bool {
	return m.Called(chartName, version).Bool(0)
}

func (m *MockIndex) MostRecentVersion(chartName string) string {
	return m.Called(chartName).String(0)
}
