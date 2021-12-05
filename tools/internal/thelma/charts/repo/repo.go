package repo

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/gcp/bucket"
	"path"
	"time"
)

const ChartDir = "charts"
const indexObject = "index.yaml"

const defaultChartCacheControl = "public, max-age=300"
const defaultIndexCacheControl = "no-cache"

const defaultLockPath = ".repo.lk"
const defaultLockWaitTimeout = 2 * time.Minute
const defaultLockStaleTimeout = 5 * time.Minute

// Repo supports interactions with GCS-based Helm repositories
type Repo interface {
	// RepoURL() returns the public URL of the repo
	RepoURL() string
	// IsLocked returns true if the repo is locked
	IsLocked() bool
	// Unlock unlocks the repository
	Unlock() error
	// Lock locks the repository
	Lock() error
	// UploadChart uploads a chart to the bucket
	UploadChart(fromPath string) error
	// UploadIndex uploads an index to the bucket
	UploadIndex(fromPath string) error
	// HasIndex returns true if this repo has an index object
	HasIndex() (bool, error)
	// DownloadIndex downloads an index file locally
	DownloadIndex(destPath string) error
}

type Options struct {
	LockWaitTimeout   time.Duration
	LockStaleTimeout  time.Duration
	LockPath          string
	ChartCacheControl string
	IndexCacheControl string
}

type repo struct {
	bucket         bucket.Bucket
	lockGeneration int64
	options        *Options
}

func DefaultOptions() *Options {
	return &Options{
		LockWaitTimeout:   defaultLockWaitTimeout,
		LockStaleTimeout:  defaultLockStaleTimeout,
		LockPath:          defaultLockPath,
		ChartCacheControl: defaultChartCacheControl,
		IndexCacheControl: defaultIndexCacheControl,
	}
}

func NewRepo(bucket bucket.Bucket) Repo {
	return &repo{
		bucket:  bucket,
		options: DefaultOptions(),
	}
}

// RepoURL returns the external URL of the Helm repository
func (r *repo) RepoURL() string {
	return fmt.Sprintf("https://%s.storage.googleapis.com", r.bucket.Name())
}

// IsLocked returns true if the repo is locked
func (r *repo) IsLocked() bool {
	return r.lockGeneration != 0
}

// Unlock unlocks the repository
func (r *repo) Unlock() error {
	if !r.IsLocked() {
		return fmt.Errorf("repo is not locked")
	}

	if err := r.bucket.ReleaseLock(r.options.LockPath, r.lockGeneration); err != nil {
		return err
	}

	r.lockGeneration = 0

	return nil
}

// Lock locks the repository
func (r *repo) Lock() error {
	if r.IsLocked() {
		return fmt.Errorf("repo is already locked")
	}

	if err := r.bucket.DeleteStaleLock(r.options.LockPath, r.options.LockStaleTimeout); err != nil {
		return err
	}

	lockGeneration, err := r.bucket.WaitForLock(r.options.LockPath, r.options.LockWaitTimeout)
	if err != nil {
		return err
	}

	r.lockGeneration = lockGeneration

	return nil
}

// UploadChart uploads a chart package file to the correct path in the bucket
func (r *repo) UploadChart(fromPath string) error {
	objectPath := path.Join(ChartDir, path.Base(fromPath))
	return r.bucket.Upload(fromPath, objectPath, r.options.ChartCacheControl)
}

// UploadIndex uploads an index file to correct path in the buck
func (r *repo) UploadIndex(fromPath string) error {
	return r.bucket.Upload(fromPath, indexObject, r.options.IndexCacheControl)
}

// HasIndex returns true if this repo has an index object
func (r *repo) HasIndex() (bool, error) {
	return r.bucket.Exists(indexObject)
}

// DownloadIndex downloads the index object to given push
func (r *repo) DownloadIndex(destPath string) error {
	return r.bucket.Download(indexObject, destPath)
}
