package resolver

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/tools/helm"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"
	"path"
)

// remoteResolver downloads charts from a Helm repo, unpacking them in the configured directory on disk.
type remoteResolver interface {
	resolve(chart ChartRelease) (ResolvedChart, error)
}

type remoteResolverImpl struct {
	cacheDir   string
	scratchDir string
	cache      syncCache
	runner     shell.Runner
}

func newRemoteResolver(cacheDir string, scratchDir string, runner shell.Runner) remoteResolver {
	return &remoteResolverImpl{
		cacheDir:   cacheDir,
		scratchDir: scratchDir,
		cache:      newSyncCache(),
		runner:     runner,
	}
}

func (r *remoteResolverImpl) resolve(chartRelease ChartRelease) (ResolvedChart, error) {
	return r.cache.get(chartRelease, r.resolverFn)
}

// Fetch the chart from the Helm repo and unpack in the cache directory
func (r *remoteResolverImpl) resolverFn(chartRelease ChartRelease) (ResolvedChart, error) {
	// Create a tmp dir for downloading and unpacking the chart
	tmpDir := path.Join(r.scratchDir, fmt.Sprintf("%s-%s-%s", chartRelease.Repo, chartRelease.Name, chartRelease.Version))
	if err := os.MkdirAll(tmpDir, 0775); err != nil {
		return nil, fmt.Errorf("failed to make tmp dir in %s: %v", r.scratchDir, err)
	}
	defer r.cleanupTmpDir(tmpDir)

	// Run `helm pull` to download the chart into tmp dir
	cmd := shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"fetch",
			path.Join(chartRelease.Repo, chartRelease.Name),
			"--version",
			chartRelease.Version,
			"--untar",
			"-d",
			tmpDir,
		},
	}

	if err := r.runner.Run(cmd); err != nil {
		return nil, fmt.Errorf("error downloading chart %s/%s version %s to %s: %v", chartRelease.Repo, chartRelease.Name, chartRelease.Version, tmpDir, err)
	}

	// Move downloaded chart to correct location in the cache directory
	// ${tmpDir}/${chart} -> ${cacheDir}/${repo}/${chart}-${version}
	cachePath := r.cachePath(chartRelease)

	files, err := ioutil.ReadDir(tmpDir)
	if err != nil {
		return nil, err
	}
	if len(files) != 1 {
		return nil, fmt.Errorf("expected exactly one file in %s, got: %v", tmpDir, files)
	}
	tmpChartPath := path.Join(tmpDir, files[0].Name())

	log.Debug().Msgf("Rename %s to %s", tmpChartPath, cachePath)
	if err = os.MkdirAll(path.Dir(cachePath), 0775); err != nil {
		return nil, err
	}
	if err = os.Rename(tmpChartPath, cachePath); err != nil {
		return nil, err
	}

	return NewResolvedChart(cachePath, chartRelease.Version, Remote, chartRelease), nil
}

// Path in the filesystem where cached chart should be kept.
// eg. "${cacheDir}/terra-helm/agora-1.2.3"
func (r *remoteResolverImpl) cachePath(chart ChartRelease) string {
	return path.Join(r.cacheDir, chart.Repo, fmt.Sprintf("%s-%s", chart.Name, chart.Version))
}

// Cleans up tmp directory, logging error instead of returning so it can be used with `defer`
func (r *remoteResolverImpl) cleanupTmpDir(tmpDir string) {
	err := os.RemoveAll(tmpDir)
	if err != nil {
		log.Warn().Msgf("Error deleting tmp dir %s: %v", tmpDir, err)
	}
}
