package source

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const fileRepoPrefix = "file://.."
const chartManifestFile = "Chart.yaml"

// ChartManifest struct used to unmarshal Helm Chart.yaml files.
type ChartManifest struct {
	Name string `yaml:"name"`
	Version string `yaml:"version"`
	Dependencies []struct {
		Name string `yaml:"name"`
		Repository string `yaml:"repository"`
	} `yaml:"dependencies"`
}

// Dir represents a directory of chart sources on the local filesystem
type Dir struct {
	charts map[string]Chart
}

// NewSourceDirectory constructor for Dir object
func NewSourceDirectory(sourceDir string, shellRunner shell.Runner) (*Dir, error) {
	// Glob inside the chart source directory for Chart.yaml files
	glob := path.Join(sourceDir, path.Join("*", chartManifestFile))
	manifestFiles, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("error globbing charts with %q: %v", glob ,err)
	}

	// For each Chart.yaml file, parse it and store in collection of Chart objects
	charts := make(map[string]Chart)

	for _, manifestFile := range manifestFiles {
		content, err := ioutil.ReadFile(manifestFile)
		if err != nil {
			return nil, fmt.Errorf("error reading chart manifest %s: %v", manifestFile, err)
		}
		manifest := ChartManifest{}
		if err := yaml.Unmarshal(content, &manifest); err != nil {
			return nil, fmt.Errorf("error parsing chart manifest %s: %v", manifestFile, err)
		}
		log.Debug().Msgf("loaded chart manifest from %s: %v", manifestFile, manifest)

		// Create node for this chart
		chart := Chart{
			name:     manifest.Name,
			path:     path.Dir(manifestFile),
			manifest: manifest,
			shellRunner: shellRunner,
		}
		charts[chart.name] = chart
	}

	return &Dir{
		charts: charts,
	}, nil
}

// ChartNames returns the names of charts in the source directory, in alphabetical order
func (sourceDir *Dir) ChartNames() []string {
	result := make([]string, 0, len(sourceDir.charts))
	for chartName := range sourceDir.charts {
		result = append(result, chartName)
	}
	sort.Strings(result)
	return result
}

// HasChart true if this source directory has a chart by the given name, false otherwise
func (sourceDir *Dir) HasChart(chartName string) bool {
	_, exists := sourceDir.charts[chartName]
	return exists
}

// GetChart given a chart name, return the associated Chart object
func (sourceDir *Dir) GetChart(chartName string) (Chart, error) {
	chart, exists := sourceDir.charts[chartName]
	if !exists {
		return chart, fmt.Errorf("unknown chart: %s", chartName)
	}
	return chart, nil
}

// LocalDependencies given the name of a chart, return the names of the charts that are its local dependencies
func (sourceDir *Dir) LocalDependencies(chartName string) ([]string, error) {
	chart, err := sourceDir.GetChart(chartName)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, dependency := range chart.manifest.Dependencies {
		log.Debug().Msgf("processing chart %s dependency: %v", chart.name, dependency)
		if !strings.HasPrefix(dependency.Repository, fileRepoPrefix) {
			log.Debug().Msgf("dependency %s is not from a %s repository, ignoring", dependency.Name, fileRepoPrefix)
			continue
		}

		if _, exists := sourceDir.charts[dependency.Name]; !exists {
			log.Warn().Msgf("dependency %s is not in source dir, ignoring", dependency.Name)
			continue
		}

		result = append(result, dependency.Name)
	}

	return result, nil
}
