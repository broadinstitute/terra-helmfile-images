package publish

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"path"
	"testing"
)

type testState struct {
	scratchDir string
	mockRepo   *repo.MockRepo
	mockRunner *shell.MockRunner
	publisher  *publisher
}

func TestPublish(t *testing.T) {
	testCases := []struct {
		description string
		test        func(ts testState)
		dryRun      bool
	}{
		{
			description: "should panic error if no charts have been added",
			dryRun:      false,
			test: func(ts testState) {
				assert.Panics(t, func() {
					_, _ = ts.publisher.Publish()
				})
			},
		},
		{
			description: "should not upload charts if dry run is true",
			dryRun:      true,
			test: func(ts testState) {
				ts.mockRepo.On("RepoURL").Return("https://fake")

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

				count, err := publisher.Publish()
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
		{
			description: "should upload charts & index if dryRun is false",
			dryRun:      false,
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

				count, err := publisher.Publish()
				assert.NoError(t, err)
				assert.Equal(t, 2, count)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockRepo := repo.NewMockRepo()
			mockRunner := shell.DefaultMockRunner()

			scratchDir := t.TempDir()
			indexFile := path.Join(scratchDir, prevIndexFile)

			if !tc.dryRun {
				mockRepo.On("Lock").Return(nil)
			}
			mockRepo.On("HasIndex").Return(true, nil)
			mockRepo.On("DownloadIndex", indexFile).Run(func(args mock.Arguments) {
				writeFakeIndexFile(t, indexFile)
			}).Return(nil)

			publisher, err := NewPublisher(mockRepo, mockRunner, scratchDir, tc.dryRun)
			assert.NoError(t, err)
			assert.NotNil(t, publisher)

			ts := testState{
				scratchDir: scratchDir,
				mockRepo:   mockRepo,
				mockRunner: mockRunner,
				publisher:  publisher,
			}
			tc.test(ts)

			mockRepo.AssertExpectations(t)
			mockRunner.AssertExpectations(t)
		})
	}
}

func TestConstructorCreatesEmptyIndexIfNoExist(t *testing.T) {
	mockRepo := repo.NewMockRepo()
	mockRunner := shell.DefaultMockRunner()

	tmpDir := t.TempDir()

	mockRepo.On("Lock").Return(nil)
	mockRepo.On("HasIndex").Return(false, nil)

	publisher, err := NewPublisher(mockRepo, mockRunner, tmpDir, false)

	assert.NoError(t, err)
	assert.NotNil(t, publisher)
	assert.Equal(t, "", publisher.Index().MostRecentVersion("foo"))

	mockRunner.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func writeFakeIndexFile(t *testing.T, path string) {
	content := `
entries:
  foo:
    - version: 1.2.3
    - version: 4.5.6
  bar:
    - version: invalid
`
	if err := os.WriteFile(path, []byte(content), 0400); err != nil {
		t.Fatal(err)
	}
}

func addFakeChart(t *testing.T, publisher *publisher, name string, version string) {
	file := path.Join(publisher.ChartDir(), fmt.Sprintf("%s-%s.tgz", name, version))
	if err := os.WriteFile(file, []byte("fake chart file"), 0400); err != nil {
		t.Fatal(err)
	}
}
