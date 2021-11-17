package repo

import (
	"errors"
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo/gcs"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo/index"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/helm"
	"github.com/rs/zerolog/log"
	"os"
	"path"
	"path/filepath"
	"time"
)

const chartsSubdir = "charts"
const chartCacheControl = "public, max-age=300"
const indexCacheControl = "no-cache"
const lockObject = ".repo-update.lk"
const lockWait = 2 * time.Minute
const lockStaleAge = 5 * time.Minute
const indexObject = "index.yaml"
const prevIndexFile = "index-prev.yaml"
const newIndexFile = "index.yaml"

// How to use ChartUploader?
// Well, we point it at a Bucket.
// All GCS Helm repos follow the structure of having a bucket with index at root and charts under charts/ path.

type ChartUploader struct {
	bucket *gcs.Bucket
	stagingDir string
	shellRunner shell.Runner
	lockGeneration int64
}

func NewUploader(bucket *gcs.Bucket, app *app.ThelmaApp) (*ChartUploader, error) {
	dir, err := app.Paths.CreateScratchDir("chart-uploader")
	if err != nil {
		return nil, fmt.Errorf("error creating scratch dir for chart uploader: %v", err)
	}
	return &ChartUploader{
		bucket:     bucket,
		stagingDir: dir,
		shellRunner: app.ShellRunner,
	}, nil
}

func (u *ChartUploader) ChartStagingDir() string {
	return path.Join(u.stagingDir, chartsSubdir)
}

// LockRepo locks the repo to prevent updates
func (u *ChartUploader) LockRepo() error {
	if u.IsRepoLocked() {
		return fmt.Errorf("repo is already locked")
	}

	if err := u.bucket.DeleteStaleLockIfExists(lockObject, lockStaleAge); err != nil {
		return err
	}
	lockGen, err := u.bucket.WaitForLock(lockObject, "chart-uploader", lockWait)
	if err != nil {
		return err
	}

	u.lockGeneration = lockGen

	return nil
}

func (u *ChartUploader) UnlockRepo() error {
	if !u.IsRepoLocked() {
		return fmt.Errorf("repo is not locked")
	}

	if err := u.bucket.ReleaseLock(lockObject, u.lockGeneration); err != nil {
		return err
	}

	u.lockGeneration = 0

	return nil
}

func (u *ChartUploader) IsRepoLocked() bool {
	return u.lockGeneration != 0
}

func (u *ChartUploader) UpdateRepo() (updatedCount int, err error) {
	if err = u.fetchCurrentIndexIfNotExists(); err != nil {
		return
	}

	if err = u.generateNewIndex(); err != nil {
		return
	}

	updatedCount, err = u.uploadCharts()
	if err != nil {
		return
	}

	if err = u.uploadIndex(); err != nil {
		return
	}

	return
}

func (u *ChartUploader) LoadIndex() (*index.Index, error) {
	if err := u.fetchCurrentIndexIfNotExists(); err != nil {
		return nil, err
	}

	return index.LoadFromFile(u.prevIndexPath())
}

func (u *ChartUploader) fetchCurrentIndexIfNotExists() error {
	_, err := os.Stat(u.prevIndexPath())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("unexpected error checking filesystem for old index file %s: %v", u.prevIndexPath(), err)
		}
		return u.fetchCurrentIndex()
	}

	return nil
}


func (u *ChartUploader) fetchCurrentIndex() error {
	exists, err := u.bucket.Exists(indexObject)
	if err != nil {
		return err
	}
	if exists {
		return u.bucket.Download(indexObject, u.prevIndexPath())
	}

	// Bucket has no index file, so create an empty stub
	log.Warn().Msgf("Bucket %s has no %s object, generating empty index file", u.bucket.Name(), indexObject)
	_, err = os.Create(u.prevIndexPath())
	return err
}

func (u *ChartUploader) generateNewIndex() error {
	cmd := shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"repo",
			"index",
			"--merge",
			u.prevIndexPath(),
			fmt.Sprintf("--url=%q", u.repoURL()),
			".",
		},
		Dir: u.stagingDir,
	}

	return u.shellRunner.Run(cmd)
}

func (u *ChartUploader) uploadIndex() error {
	return u.bucket.Upload(u.newIndexPath(), indexObject, indexCacheControl)
}

func (u *ChartUploader) uploadCharts() (int, error) {
	count := 0

	glob := path.Join(u.ChartStagingDir(), "*.tgz")
	chartFiles, err := filepath.Glob(glob)
	if err != nil {
		return count, fmt.Errorf("error globbing charts with %q: %v", glob ,err)
	}

	for _, chartFile := range chartFiles {
		objectPath := path.Join(chartsSubdir, path.Base(chartFile))
		if err := u.bucket.Upload(chartFile, objectPath, chartCacheControl); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

func (u *ChartUploader) repoURL() string {
	return fmt.Sprintf("https://%s.storage.googleapis.com", u.bucket.Name())
}

func (u *ChartUploader) prevIndexPath() string {
	return path.Join(u.stagingDir, prevIndexFile)
}

func (u *ChartUploader) newIndexPath() string {
	return path.Join(u.stagingDir, newIndexFile)
}
