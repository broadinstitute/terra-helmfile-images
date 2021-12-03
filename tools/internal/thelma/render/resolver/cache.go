package resolver

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/tools/helm"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
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
	cacheDir     string
	runner       shell.Runner
	globalLock   sync.Mutex
	cacheEntries map[ChartRelease]*cacheEntry
}

type cacheEntry struct {
	alreadyFetched bool
	err            error
	cachePath      string
	mutex          *sync.Mutex
}

func NewChartCache(cacheDir string, runner shell.Runner) ChartCache {
	return &chartCache{
		cacheDir:     cacheDir,
		runner:       runner,
		cacheEntries: make(map[ChartRelease]*cacheEntry),
	}
}

func (c *chartCache) Fetch(chart ChartRelease) (string, error) {
	entry := c.getCacheEntry(chart)
	entry.mutex.Lock()
	defer entry.mutex.Unlock()

	if entry.alreadyFetched {
		log.Debug().Msgf("Chart %s/%s version %s already fetched, returning cached result: [%s, %v]", chart.Repo, chart.Name, chart.Version, entry.cachePath, entry.err)
		return entry.cachePath, entry.err
	}

	cachePath, err := c.tryFetch(chart)

	entry.cachePath = cachePath
	entry.err = err
	entry.alreadyFetched = true

	return entry.cachePath, entry.err
}

// Tries to fetch the chart (no locking / safety)
func (c *chartCache) tryFetch(chart ChartRelease) (string, error) {
	cachePath := c.cachePath(chart)

	tmpDir := siblingPath(cachePath, ".tmp", true)

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
func (c *chartCache) getCacheEntry(chart ChartRelease) *cacheEntry {
	c.globalLock.Lock()
	defer c.globalLock.Unlock()

	_, exists := c.cacheEntries[chart]
	if !exists {
		c.cacheEntries[chart] = &cacheEntry{
			mutex: &sync.Mutex{},
		}
	}

	return c.cacheEntries[chart]
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
