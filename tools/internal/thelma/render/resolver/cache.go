package resolver

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/tools/helm"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"
	"path"
	"sync"
)

// ChartCache caches unpacked charts on disk
type ChartCache interface {
	Fetch(chart ChartRelease) (string, error)
}

type chartCache struct {
	cacheDir   string
	runner     shell.Runner
	globalLock sync.Mutex
	cacheLocks map[ChartRelease]*sync.Mutex
}

func NewChartCache(cacheDir string, runner shell.Runner) ChartCache {
	return &chartCache{
		cacheDir:   cacheDir,
		runner:     runner,
		cacheLocks: make(map[ChartRelease]*sync.Mutex),
	}
}

func (c *chartCache) Fetch(chart ChartRelease) (string, error) {
	cachePath := c.cachePath(chart)

	lock, exists := c.getCacheLock(chart)
	if exists {
		return cachePath, nil
	}

	lock.Lock()
	defer lock.Unlock()

	tmpDir := siblingPath(cachePath, ".tmp", true)
	log.Info().Msgf("Downloading chart to tmp dir %s", tmpDir)

	cmd := shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"fetch",
			path.Join(chart.Repo, chart.Name),
			"--version",
			chart.Version,
			"--untar",
			"-d",
			tmpDir,
		},
	}

	if err := c.runner.Run(cmd); err != nil {
		return "", fmt.Errorf("error downloading chart %s/%s version %s to %s: %v", chart.Repo, chart.Name, chart.Version, tmpDir, err)
	}

	// helm fetch repo/chart --untar -d /mydir will nest the chart one level, so we'll end up
	// with /mydir/<chart>/Chart.yaml, for example.
	// But we _want_ /mydir/Chart.yaml, so we remove the extra level of nesting.
	files, err := ioutil.ReadDir(tmpDir)
	if err != nil {
		return "", err
	}
	if len(files) != 1 {
		return "", fmt.Errorf("expected exactly one file in %s, got: %v", tmpDir, files)
	}

	tmpChartPath := path.Join(tmpDir, files[0].Name())
	log.Debug().Msgf("Rename %s to %s", tmpChartPath, cachePath)
	if err = os.Rename(tmpChartPath, cachePath); err != nil {
		return "", err
	}

	if err = os.RemoveAll(tmpDir); err != nil {
		return "", err
	}

	return cachePath, nil
}

// Returns lock for the given chart name, and a boolean (true if a lock already existed, false otherwise)
func (c *chartCache) getCacheLock(chart ChartRelease) (*sync.Mutex, bool) {
	c.globalLock.Lock()
	defer c.globalLock.Unlock()

	_, exists := c.cacheLocks[chart]
	if !exists {
		c.cacheLocks[chart] = &sync.Mutex{}
	}

	return c.cacheLocks[chart], exists
}

// eg. "terra-helm/agora-1.2.3"
func (c *chartCache) cachePath(chart ChartRelease) string {
	return path.Join(c.cacheDir, chart.Repo, fmt.Sprintf("%s-%s", chart.Name, chart.Version))
}

// siblingPath is a path utility function. Given a directory path,
// it returns a path in the same parent directory, with the same basename,
// with a user-supplied suffix. The new path may optionally be hidden.
//
// Eg.
//  siblingPath("/tmp/foo", ".lk", true)
// ->
// "/tmp/.foo.lk"
//
// siblingPath("/tmp/foo", "-scratch", false)
// ->
// "/tmp/foo-scratch"
func siblingPath(relpath string, suffix string, hidden bool) string {
	cleaned := path.Clean(relpath)
	parent := path.Dir(cleaned)
	base := path.Base(cleaned)

	var prefix string
	if hidden {
		prefix = "."
	}

	siblingBase := fmt.Sprintf("%s%s%s", prefix, base, suffix)
	return path.Join(parent, siblingBase)
}
