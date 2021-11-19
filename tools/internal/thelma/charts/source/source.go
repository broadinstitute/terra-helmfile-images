package source

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/rs/zerolog/log"
	"path"
	"path/filepath"
	"sort"
)

// Dir represents a directory of chart sources on the local filesystem
type Dir struct {
	charts map[string]Chart
}

// NewSourceDirectory constructor for Dir object
func NewSourceDirectory(sourceDir string, shellRunner shell.Runner) (*Dir, error) {
	// Glob inside the chart source directory for chart.yaml files
	glob := path.Join(sourceDir, path.Join("*", chartManifestFile))
	manifestFiles, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("error globbing charts with %q: %v", glob, err)
	}

	// For each chart.yaml file, parse it and store in collection of chart objects
	charts := make(map[string]Chart)

	for _, manifestFile := range manifestFiles {
		// Create node for this chart
		_chart, err := NewChart(manifestFile, shellRunner)
		if err != nil {
			return nil, fmt.Errorf("error creating chart from %s: %v", manifestFile, err)
		}
		charts[_chart.Name()] = _chart
	}

	return &Dir{
		charts: charts,
	}, nil
}

// ChartNames returns the names of charts in the source directory, in alphabetical order
func (dir *Dir) ChartNames() []string {
	result := make([]string, 0, len(dir.charts))
	for chartName := range dir.charts {
		result = append(result, chartName)
	}
	sort.Strings(result)
	return result
}

// HasChart true if this source directory has a chart by the given name, false otherwise
func (dir *Dir) HasChart(chartName string) bool {
	_, exists := dir.charts[chartName]
	return exists
}

// GetChart given a chart name, return the associated chart object
func (dir *Dir) GetChart(chartName string) (Chart, error) {
	_chart, exists := dir.charts[chartName]
	if !exists {
		return _chart, fmt.Errorf("unknown chart: %s", chartName)
	}
	return _chart, nil
}

// LocalDependencies returns a map of chart names keyed to a list of their local dependencies
func (dir *Dir) LocalDependencies() map[string][]string {
	deps := make(map[string][]string)
	for chartName, _chart := range dir.charts {
		var localDeps []string
		for _, depName := range _chart.LocalDependencies() {
			// double-check that the dependencies eactually exist in the chart dir
			if _, exists := dir.charts[depName]; !exists {
				log.Warn().Msgf("chart %s dependency %s is not in source dir, ignoring", _chart.Name(), depName)
				continue
			}
			localDeps = append(localDeps, depName)
		}
		deps[chartName] = localDeps
	}
	return deps
}
