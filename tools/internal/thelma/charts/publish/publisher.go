package publish

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo/index"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/tools/helm"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/rs/zerolog/log"
	"os"
	"path"
	"path/filepath"
)

// Publisher is a utility for publishing Helm charts to a GCS bucket-hosted Helm repository.
type Publisher interface {
	// ChartDir returns the path where new chart packages should be copied for upload
	ChartDir() string
	// Index returns a queryable copy of th eindex that is currently in the Helm repo bucket
	Index() index.Index
	// Publish uploads all charts in the chart directory to the target Helm repo.
	// Can only be called once for a given publisher instance!
	Publish() (count int, err error)
	// Close unlocks the repo and releases all resources associated with this publisher
	Close() error
}

// implements Publisher interface
type publisher struct {
	repo        repo.Repo
	stagingDir  *stagingDir
	shellRunner shell.Runner
	index       index.Index
	dryRun      bool
	closed      bool
}

// NewPublisher is a constructor for a publisher
func NewPublisher(repo repo.Repo, runner shell.Runner, scratchDir string, dryRun bool) (*publisher, error) {
	_stagingDir := &stagingDir{root: scratchDir}

	if err := os.Mkdir(_stagingDir.chartDir(), 0755); err != nil {
		return nil, fmt.Errorf("chart-publisher: failed to create chart dir: %v", err)
	}

	if dryRun {
		log.Warn().Msgf("chart-publisher: not locking repo, this is a dry run")
	} else if err := repo.Lock(); err != nil {
		return nil, fmt.Errorf("chart-publisher: error locking repo: %v", err)
	}

	_index, err := initializeIndex(repo, _stagingDir)
	if err != nil {
		// unlock repo, since we won't be using it
		if err2 := repo.Unlock(); err2 != nil {
			log.Error().Msgf("chart-publisher: failed to unlock repo: %v", err2)
		}
		return nil, err
	}

	return &publisher{
		repo:        repo,
		stagingDir:  _stagingDir,
		index:       _index,
		shellRunner: runner,
		dryRun:      dryRun,
		closed:      false,
	}, nil
}

// ChartDir returns the path where new chart packages should be copied for upload
func (u *publisher) ChartDir() string {
	return u.stagingDir.chartDir()
}

// Index returns a queryable version the index that is currently in the bucket.
func (u *publisher) Index() index.Index {
	return u.index
}

// Publish publishes all charts in the chart directory to the target Helm repo,
// returning an integer denoting the number of charts that were published.
func (u *publisher) Publish() (int, error) {
	if u.closed {
		panic("Publish() can only be called once")
	}

	chartFiles, err := u.listChartFiles()
	if err != nil {
		return -1, err
	}

	if len(chartFiles) == 0 {
		panic("at least one chart must be added to chart directory before Publish() is called")
	}

	if err := u.generateNewIndex(); err != nil {
		return -1, fmt.Errorf("chart-publisher: failed to generate new index file: %v", err)
	}

	if u.dryRun {
		log.Warn().Msgf("chart-publisher: not uploading any charts, this is a dry run")
		return 0, u.Close()
	}

	for _, chartFile := range chartFiles {
		if err := u.uploadChart(chartFile); err != nil {
			return -1, fmt.Errorf("chart-publisher: error uploading chart %s: %v", chartFile, err)
		}
	}

	if err := u.uploadIndex(); err != nil {
		return -1, fmt.Errorf("chart-publisher: error uploading index: %v", err)
	}

	return len(chartFiles), u.Close()
}

// Close releases all resources associated with this uploader instance. This includes:
// * unlocking the Helm repo
// * deleting the chart staging directory
func (u *publisher) Close() error {
	if u.closed {
		return nil
	}

	if u.dryRun {
		log.Warn().Msgf("chart-publisher: not unlocking repo, this is a dry run")
	} else if err := u.repo.Unlock(); err != nil {
		return fmt.Errorf("chart-publisher: error unlocking repo: %v", err)
	}

	if err := os.RemoveAll(u.stagingDir.root); err != nil {
		return fmt.Errorf("chart-publisher: error cleaning up staging dir: %v", err)
	}

	u.closed = true
	return nil
}

// Generate a new index file that includes the updated charts
func (u *publisher) generateNewIndex() error {
	cmd := shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"repo",
			"index",
			"--merge",
			u.stagingDir.prevIndexFile(),
			"--url",
			u.repo.RepoURL(),
			".",
		},
		Dir: u.stagingDir.root,
	}

	return u.shellRunner.Run(cmd)
}

// Upload the new index file
func (u *publisher) uploadIndex() error {
	log.Info().Msgf("chart-publisher: Uploading new index to repo")
	return u.repo.UploadIndex(u.stagingDir.newIndexFile())
}

// Upload a new chart file
func (u *publisher) uploadChart(localPath string) error {
	log.Info().Msgf("chart-publisher: Uploading chart %s to repo", localPath)
	return u.repo.UploadChart(localPath)
}

// Return a list of chart packages in the chart directory
func (u *publisher) listChartFiles() ([]string, error) {
	glob := path.Join(u.ChartDir(), "*.tgz")
	chartFiles, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("chart-publisher: error globbing charts with %q: %v", glob, err)
	}

	return chartFiles, nil
}

// Populate a new Index object from the repo, or create an empty index if the repo doesn't have one
func initializeIndex(repo repo.Repo, stagingDir *stagingDir) (index.Index, error) {
	exists, err := repo.HasIndex()
	if err != nil {
		return nil, fmt.Errorf("chart-publisher: error downloading index from repo: %v", err)
	}
	if exists {
		if err := repo.DownloadIndex(stagingDir.prevIndexFile()); err != nil {
			return nil, fmt.Errorf("chart-publisher: error downloading index from repo: %v", err)
		}
	} else {
		log.Warn().Msgf("chart-publisher: repo has no index object, generating empty index file")
		_, err = os.Create(stagingDir.prevIndexFile())
		if err != nil {
			return nil, fmt.Errorf("chart-publisher: error creating empty index file: %v", err)
		}
	}

	return index.LoadFromFile(stagingDir.prevIndexFile())
}
