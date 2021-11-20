package source

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/dependency"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/publish"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/versions"
	"github.com/rs/zerolog/log"
	"path"
	"path/filepath"
	"strings"
)

// ChartsDir represents a directory of Helm chart sources on the local filesystem.
type ChartsDir interface {
	Release(chartNames []string) error
}

// NewChartsDir constructs a new ChartsDir
func NewChartsDir(
	sourceDir string,
	publisher publish.Publisher,
	versions versions.Versions,
	shellRunner shell.Runner,
	) (ChartsDir, error) {

	charts, err := loadCharts(sourceDir, shellRunner)
	if err != nil {
		return nil, err
	}

	dependencyGraph, err := buildDependencyGraph(charts)
	if err != nil {
		return nil, err
	}

	return &chartsDir{
		sourceDir: sourceDir,
		charts: charts,
		publisher: publisher,
		autoreleaser: NewAutoReleaser(versions),
		dependencyGraph: dependencyGraph,
	}, nil
}

// implemeents ChartsDir interface
type chartsDir struct {
	sourceDir string
	charts map[string]Chart
	publisher publish.Publisher
	autoreleaser AutoReleaser
	dependencyGraph *dependency.Graph
}

func (d *chartsDir) Release(chartNames []string) error {
	chartsToPublish := chartNames
	for _, chartName := range chartsToPublish {
		if _, exists := d.charts[chartName]; !exists {
			return fmt.Errorf("chart %q does not exist in source dir %s", chartName, d.sourceDir)
		}
	}

	// Add dependents.
	chartsToPublish = d.withDependents(chartsToPublish)

	d.dependencyGraph.TopoSort(chartsToPublish)
	log.Info().Msgf("%d charts will be published: %s", len(chartsToPublish), strings.Join(chartsToPublish, ","))

	for _, chartName := range chartsToPublish {
		_chart := d.charts[chartName]

		if err := _chart.GenerateDocs(); err != nil {
			return err
		}
		newVersion, err := _chart.BumpChartVersion(d.publisher.Index().MostRecentVersion(chartName))
		if err != nil {
			return err
		}
		if err := _chart.BuildDependencies(); err != nil {
			return err
		}
		if err := _chart.PackageChart(d.publisher.ChartDir()); err != nil {
			return err
		}
		if err := d.autoreleaser.UpdateVersionsFile(_chart, newVersion); err != nil {
			return err
		}
	}

	count, err := d.publisher.Publish()
	if err != nil {
		return err
	}

	log.Info().Msgf("Published %d charts", count)

	return nil
}

func (d *chartsDir) withDependents(chartNames []string) []string {
	withDeps := d.dependencyGraph.WithDependents(chartNames...)

	diff := len(withDeps) - len(chartNames)
	if diff > 0 {
		log.Info().Msgf("Identified %d additional downstream charts to publish", diff)
	}

	return withDeps
}

func buildDependencyGraph(charts map[string]Chart) (*dependency.Graph, error) {
	dependencies := make(map[string][]string)
	for chartName, _chart := range charts {
		var localDeps []string
		for _, depName := range _chart.LocalDependencies() {
			// double-check that the dependencies actually exist in the chart dir
			if _, exists := charts[depName]; !exists {
				log.Warn().Msgf("chart %s dependency %s is not in source dir, ignoring", _chart.Name(), depName)
				continue
			}
			localDeps = append(localDeps, depName)
		}
		dependencies[chartName] = localDeps
	}

	return dependency.NewGraph(dependencies)
}

func loadCharts(sourceDir string, shellRunner shell.Runner) (map[string]Chart, error) {
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

	return charts, nil
}