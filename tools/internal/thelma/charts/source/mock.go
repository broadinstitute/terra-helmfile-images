package source

import "github.com/stretchr/testify/mock"

type MockChart struct {
	mock.Mock
}

func NewMockChart() *MockChart {
	return &MockChart{}
}

func (c *MockChart) Name() string {
	return c.Called().String(0)
}

func (c *MockChart) Path() string {
	return c.Called().String(0)
}

func (c *MockChart) BumpChartVersion(latestPublishedVersion string) (string, error) {
	result := c.Called(latestPublishedVersion)
	return result.String(0), result.Error(1)
}

func (c *MockChart) UpdateDependencies() error {
	return c.Called().Error(0)
}

func (c *MockChart) PackageChart(destPath string) error {
	return c.Called(destPath).Error(0)
}

func (c *MockChart) GenerateDocs() error {
	return c.Called().Error(0)
}

func (c *MockChart) LocalDependencies() []string {
	return c.Called().Get(0).([]string)
}

func (c *MockChart) SetDependencyVersion(dependencyName string, newVersion string) error {
	return c.Called(dependencyName, newVersion).Error(0)
}

func (c *MockChart) ManifestVersion() string {
	return c.Called().String(0)
}
