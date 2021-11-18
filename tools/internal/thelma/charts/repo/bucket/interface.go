package bucket

import (
	"time"
)

// Bucket offers higher-level operations on GCS buckets
type Bucket interface {
	// Name returns the name of the bucket
	Name() string

	// Close closes gcs client associated with this bucket
	Close() error

	// WaitForLock waits for a lock, timing out after maxTime. It returns a lock id / object generation number
	// that must be passed in to ReleaseLock
	WaitForLock(objectPath string, maxWait time.Duration) (int64, error)

	// DeleteStaleLock deletes a stale lock file if it exists and is older than staleAge
	DeleteStaleLock(objectPath string, staleAge time.Duration) error

	// ReleaseLock removes a lockfile
	ReleaseLock(objectPath string, generation int64) error

	// Exists returns true if the object exists, false otherwise
	Exists(objectPath string) (bool, error)

	// Upload uploads a local file to the bucket
    Upload(localPath string, objectPath string, cacheControl string) error

	// Download downloads an object in the bucket to a local file
	Download(objectPath string, localPath string) error
}
