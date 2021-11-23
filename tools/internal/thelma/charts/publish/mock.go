package publish

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo/index"
	"github.com/stretchr/testify/mock"
)

type MockPublisher struct {
	mock.Mock
}

func NewMockPublisher() *MockPublisher {
	return &MockPublisher{}
}

func (m *MockPublisher) ChartDir() string {
	return m.Called().String(0)
}

func (m *MockPublisher) Index() index.Index {
	return m.Called().Get(0).(index.Index)
}

func (m *MockPublisher) Publish() (count int, err error) {
	r := m.Called()
	return r.Int(0), r.Error(1)
}

func (m *MockPublisher) Close() error {
	return m.Called().Error(0)
}
