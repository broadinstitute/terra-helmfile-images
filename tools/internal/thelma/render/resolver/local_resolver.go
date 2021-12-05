package resolver

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/source"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"os"
	"path"
)

// LocalResolver is for resolving charts in local directory of chart sources.
// (i.e. the charts/ subdirectory in terra-helmfile).
type localResolver interface {
	// True if the chart exists in the source directory, else false.
	// Returns error in the event of an unexpected filesystem i/o error
	chartExists(chart ChartRelease) (bool, error)
	// Locates the chart in the source directory, running `helm dependency update` on it if needed
	resolve(chart ChartRelease) (ResolvedChart, error)
	// Returns version of the chart in the chart's Chart.yaml
	sourceVersion(chart ChartRelease) (string, error)
}

type localResolverImpl struct {
	sourceDir string
	cache     syncCache
	runner    shell.Runner
}

func newLocalResolver(sourceDir string, runner shell.Runner) localResolver {
	cache := newSyncCacheWithMapper(func(chart ChartRelease) string {
		// since all charts live at the first level of the source directory
		// eg. charts/agora, charts/cromwell, etc.
		// we key on chart name and ignore version and repository
		return chart.Name
	})

	return &localResolverImpl{
		sourceDir: sourceDir,
		cache:     cache,
		runner:    runner,
	}
}

func (r *localResolverImpl) chartExists(chartRelease ChartRelease) (bool, error) {
	chartPath := r.chartSourcePath(chartRelease.Name)
	_, err := os.Stat(chartPath)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, fmt.Errorf("error checking for chart at %s: %v", chartPath, err)
		}
	}

	return true, nil
}

func (r *localResolverImpl) sourceVersion(chartRelease ChartRelease) (string, error) {
	chart, err := r.getChart(chartRelease.Name)
	if err != nil {
		return "", fmt.Errorf("error reading chart %s: %v", chartRelease.Name, err)
	}

	return chart.ManifestVersion(), nil
}

func (r *localResolverImpl) resolve(chartRelease ChartRelease) (ResolvedChart, error) {
	return r.cache.get(chartRelease, r.resolverFn)
}

func (r *localResolverImpl) resolverFn(chartRelease ChartRelease) (ResolvedChart, error) {
	chart, err := r.getChart(chartRelease.Name)
	if err != nil {
		return nil, err
	}

	err = chart.UpdateDependencies()
	if err != nil {
		return nil, fmt.Errorf("error updating chart source directory %s: %v", chart.Path(), err)
	}

	return NewResolvedChart(chart.Path(), chart.ManifestVersion(), Local, chartRelease), nil
}

// Returns source.Chart instance for the given chart name
func (r *localResolverImpl) getChart(chartName string) (source.Chart, error) {
	chart, err := source.NewChart(r.chartSourcePath(chartName), r.runner)
	if err != nil {
		return nil, fmt.Errorf("error reading chart %s in %s: %v", chartName, r.sourceDir, err)
	}
	return chart, nil
}

// Returns the path to chart source directory
func (r *localResolverImpl) chartSourcePath(chartName string) string {
	return path.Join(r.sourceDir, chartName)
}
