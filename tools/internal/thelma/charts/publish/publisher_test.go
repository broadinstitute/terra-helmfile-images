package publish

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/publish/index"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"testing"
)

type testState struct {
	scratchDir string
	mockRepo   *repo.MockRepo
	mockRunner *shellmock.MockRunner
	publisher  *ChartPublisher
}

func TestPublish(t *testing.T) {
	testCases := []struct {
		description string
		test        func(ts testState)
	}{
		{
			description: "should panic error if no charts have been added",
			test: func(ts testState) {
				assert.Panics(t, func() {
					_, _ = ts.publisher.Publish(true)
				})
			},
		},
		{
			description: "should not upload charts if commit is false",
			test: func(ts testState) {
				ts.mockRepo.On("RepoURL").Return("https://fake")
				ts.mockRepo.On("Unlock").Return(nil)

				ts.mockRunner.ExpectCmd(shell.Command{
					Prog: "helm",
					Args: []string{
						"repo",
						"index",
						"--merge",
						path.Join(ts.scratchDir, prevIndexFile),
						"--url",
						"https://fake",
						".",
					},
					Dir: ts.scratchDir,
				})

				publisher := ts.publisher
				addFakeChart(t, publisher, "mychart", "0.0.1")

				count, err := publisher.Publish(false)
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
		{
			description: "should upload charts if commit is true",
			test: func(ts testState) {
				ts.mockRepo.On("RepoURL").Return("https://fake")
				ts.mockRepo.On("UploadChart", path.Join(ts.scratchDir, "charts", "charta-0.0.1.tgz")).Return(nil)
				ts.mockRepo.On("UploadChart", path.Join(ts.scratchDir, "charts", "chartb-4.5.6.tgz")).Return(nil)
				ts.mockRepo.On("UploadIndex", path.Join(ts.scratchDir, newIndexFile)).Return(nil)
				ts.mockRepo.On("Unlock").Return(nil)

				ts.mockRunner.ExpectCmd(shell.Command{
					Prog: "helm",
					Args: []string{
						"repo",
						"index",
						"--merge",
						path.Join(ts.scratchDir, prevIndexFile),
						"--url",
						"https://fake",
						".",
					},
					Dir: ts.scratchDir,
				})

				publisher := ts.publisher
				addFakeChart(t, publisher, "charta", "0.0.1")
				addFakeChart(t, publisher, "chartb", "4.5.6")

				count, err := publisher.Publish(true)
				assert.NoError(t, err)
				assert.Equal(t, 2, count)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			ts := setup(t)
			tc.test(ts)
		})
	}
}

func TestGetLatestVersion(t *testing.T) {
	ts := setup(t)
	publisher := ts.publisher

	assert.Equal(t, "4.5.6", publisher.LastPublishedVersion("foo"))
	assert.Equal(t, "", publisher.LastPublishedVersion("bar"))
	assert.Equal(t, "", publisher.LastPublishedVersion("baz"))
}

func TestConstructorCreatesEmptyIndexIfNoExist(t *testing.T) {
	mockRepo := repo.NewMockRepo()
	mockRunner := shellmock.DefaultMockRunner()

	tmpDir := t.TempDir()

	mockRepo.On("Lock").Return(nil)
	mockRepo.On("HasIndex").Return(false, nil)

	publisher, err := NewPublisher(mockRepo, mockRunner, tmpDir)

	assert.NoError(t, err)
	assert.NotNil(t, publisher)
	assert.Equal(t, "", publisher.LastPublishedVersion("foo"))

	mockRunner.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func setup(t *testing.T) testState {
	mockRepo := repo.NewMockRepo()
	mockRunner := shellmock.DefaultMockRunner()

	scratchDir := t.TempDir()
	indexFile := path.Join(scratchDir, prevIndexFile)

	mockRepo.On("Lock").Return(nil)
	mockRepo.On("HasIndex").Return(true, nil)
	mockRepo.On("DownloadIndex", indexFile).Run(func(args mock.Arguments) {
		writeFakeIndexFile(t, indexFile)
	}).Return(nil)

	publisher, err := NewPublisher(mockRepo, mockRunner, scratchDir)
	assert.NoError(t, err)
	assert.NotNil(t, publisher)

	t.Cleanup(func() {
		mockRepo.AssertExpectations(t)
		mockRunner.AssertExpectations(t)
	})

	return testState{
		scratchDir: scratchDir,
		mockRepo:   mockRepo,
		mockRunner: mockRunner,
		publisher:  publisher,
	}
}

func writeFakeIndexFile(t *testing.T, path string) {
	fakeIndex := &index.Index{
		Entries: map[string][]index.Entry{
			"foo": {
				{Version: "1.2.3"},
				{Version: "4.5.6"},
			},
			"bar": {
				{Version: "invalid"},
			},
		},
	}

	data, err := yaml.Marshal(fakeIndex)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0400); err != nil {
		t.Fatal(err)
	}
}

func addFakeChart(t *testing.T, publisher *ChartPublisher, name string, version string) {
	file := path.Join(publisher.ChartDir(), fmt.Sprintf("%s-%s.tgz", name, version))
	if err := os.WriteFile(file, []byte("fake chart file"), 0400); err != nil {
		t.Fatal(err)
	}
}
