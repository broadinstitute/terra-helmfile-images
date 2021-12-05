package repo

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/gcp/bucket"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDownloadIndex(t *testing.T) {
	mock := bucket.NewMockBucket("test-bucket")
	repo := NewRepo(mock)
	indexFile := "index.yaml"

	mock.On("Download", indexObject, indexFile).Return(nil)
	assert.NoError(t, repo.DownloadIndex(indexFile))

	mock.AssertExpectations(t)
}

func TestHasIndex(t *testing.T) {
	mock := bucket.NewMockBucket("test-bucket")
	repo := NewRepo(mock)

	mock.On("Exists", indexObject).Return(true, nil)
	result, err := repo.HasIndex()
	assert.NoError(t, err)
	assert.True(t, result)

	mock.AssertExpectations(t)
}

func TestUploadIndex(t *testing.T) {
	mock := bucket.NewMockBucket("test-bucket")
	repo := NewRepo(mock)
	indexFile := "index.yaml"
	options := DefaultOptions()

	mock.On("Upload", indexFile, indexObject, options.IndexCacheControl).Return(nil)
	assert.NoError(t, repo.UploadIndex(indexFile))

	mock.AssertExpectations(t)
}

func TestUploadChart(t *testing.T) {
	options := DefaultOptions()
	mock := bucket.NewMockBucket("test-bucket")
	repo := NewRepo(mock)
	chartFile := "path/to/chart.tgz"

	mock.On("Upload", chartFile, "charts/chart.tgz", options.ChartCacheControl).Return(nil)
	assert.NoError(t, repo.UploadChart(chartFile))

	mock.AssertExpectations(t)
}

func TestLocking(t *testing.T) {
	options := DefaultOptions()
	mock := bucket.NewMockBucket("test-bucket")
	repo := NewRepo(mock)

	mock.On("DeleteStaleLock", options.LockPath, options.LockStaleTimeout).Return(nil)
	mock.On("WaitForLock", options.LockPath, options.LockWaitTimeout).Return(int64(1337), nil)
	mock.On("ReleaseLock", options.LockPath, int64(1337)).Return(nil)

	assert.False(t, repo.IsLocked())
	assert.NoError(t, repo.Lock())
	assert.True(t, repo.IsLocked())
	assert.NoError(t, repo.Unlock())
	assert.False(t, repo.IsLocked())

	mock.AssertExpectations(t)
}

func TestRepoURL(t *testing.T) {
	mock := bucket.NewMockBucket("test-bucket")
	repo := NewRepo(mock)
	assert.Equal(t, "https://test-bucket.storage.googleapis.com", repo.RepoURL())
}
