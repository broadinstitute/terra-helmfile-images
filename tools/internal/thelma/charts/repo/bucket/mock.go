package bucket

import (
	"github.com/stretchr/testify/mock"
	"time"
)

type MockBucket struct {
	name string
	mock.Mock
}

// NewMockBucket creates a new MockBucket
func NewMockBucket(name string) *MockBucket {
	return &MockBucket{name: name}
}

// Close closes gcs client associated with this bucket
func (bucket *MockBucket) Close() error {
	result := bucket.Mock.Called()
	return result.Error(0)
}

// Name returns the name of this bucket
func (bucket *MockBucket) Name() string {
	return bucket.name
}

// WaitForLock waits for a lock, timing out after maxTime. It returns a lock id / object generation number
// that must be passed in to ReleaseLock
func (bucket *MockBucket) WaitForLock(objectPath string, maxWait time.Duration) (int64, error) {
	result := bucket.Mock.Called(objectPath, maxWait)
	return result.Get(0).(int64), result.Error(1)
}

// DeleteStaleLock deletes a stale lock file if it exists and is older than staleAge
func (bucket *MockBucket) DeleteStaleLock(objectPath string, staleAge time.Duration) error {
	result := bucket.Mock.Called(objectPath, staleAge)
	return result.Error(0)
}

// ReleaseLock removes a lockfile
func (bucket *MockBucket) ReleaseLock(objectPath string, generation int64) error {
	result := bucket.Mock.Called(objectPath, generation)
	return result.Error(0)
}

// Delete deletes an object in the bucket
func (bucket *MockBucket) Delete(objectPath string) error {
	result := bucket.Mock.Called(objectPath)
	return result.Error(0)
}

// Exists returns true if the object exists, false otherwise
func (bucket *MockBucket) Exists(objectPath string) (bool, error) {
	result := bucket.Mock.Called(objectPath)
	return result.Bool(0), result.Error(1)
}

// Upload uploads a local file to the bucket
func (bucket *MockBucket) Upload(localPath string, objectPath string, cacheControl string) error {
	result := bucket.Mock.Called(localPath, objectPath, cacheControl)
	return result.Error(0)
}

// Download downloads an object in the bucket to a local file
func (bucket *MockBucket) Download(objectPath string, localPath string) error {
	result := bucket.Mock.Called(objectPath, localPath)
	return result.Error(0)
}
