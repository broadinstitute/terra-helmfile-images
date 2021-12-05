package source

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/semver"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/tools/helm"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/tools/yq"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"path"
	"strings"
)

// initial version that is assigned to brand-new charts
const initialChartVersion = "0.1.0"

// binary used for running Helm Docs
const helmDocsProg = "helm-docs"

// repository prefix for local dependencies
const fileRepoPrefix = "file://.."

// name of Helm's chart manifest file
const chartManifestFile = "Chart.yaml"

// ChartManifest struct used to unmarshal Helm chart.yaml files.
type ChartManifest struct {
	Name         string
	Version      string
	Dependencies []struct {
		Name       string
		Repository string
		Version    string
	}
}

// Chart represents a Helm chart source directory on the local filesystem.
type Chart interface {
	// Name returns the name of this chart
	Name() string
	// Path returns the path to this chart on disk
	Path() string
	// BumpChartVersion updates chart version in chart.yaml
	BumpChartVersion(latestPublishedVersion string) (string, error)
	// UpdateDependencies runs `helm dependency update` on the local copy of the chart.
	UpdateDependencies() error
	// PackageChart runs `helm package` to package a chart
	PackageChart(destPath string) error
	// GenerateDocs re-generates README documentation for the given chart
	GenerateDocs() error
	// LocalDependencies returns the names of local dependencies / subcharts (using Helm's "file://" repo support)
	LocalDependencies() []string
	// SetDependencyVersion sets the version of a dependency in this chart's Chart.yaml
	SetDependencyVersion(dependencyName string, newVersion string) error
	// ManifestVersion returns the version of this chart in Chart.yaml
	ManifestVersion() string
}

// Implements Chart interface
type chart struct {
	name        string        // name of the chart
	path        string        // path to the chart directory on the local filesystem
	manifest    ChartManifest // manifest parsed subset of chart.yaml
	shellRunner shell.Runner  // shell runner instance to use for executing commands
}

// NewChart constructs a Chart
func NewChart(chartSourceDir string, shellRunner shell.Runner) (Chart, error) {
	manifestFile := path.Join(chartSourceDir, chartManifestFile)
	manifest, err := loadManifest(manifestFile)
	if err != nil {
		return nil, err
	}

	return &chart{
		name:        manifest.Name,
		path:        path.Dir(manifestFile),
		manifest:    manifest,
		shellRunner: shellRunner,
	}, nil
}

// Name of thist chart
func (c *chart) Name() string {
	return c.name
}

// Path to the chart on the filesystem
func (c *chart) Path() string {
	return c.path
}

// BumpChartVersion update chart version in chart.yaml
func (c *chart) BumpChartVersion(latestPublishedVersion string) (string, error) {
	nextVersion := c.nextVersion(latestPublishedVersion)
	expression := fmt.Sprintf(".version = %q", nextVersion)
	manifestFile := path.Join(c.path, chartManifestFile)
	if err := yq.New(c.shellRunner).Write(expression, manifestFile); err != nil {
		return nextVersion, err
	}
	if err := c.reloadManifest(); err != nil {
		return nextVersion, err
	}
	if c.manifest.Version != nextVersion {
		return nextVersion, fmt.Errorf("error updating %s chart version to %s in %s: version is still %s after update", c.name, nextVersion, manifestFile, c.manifest.Version)
	}
	return nextVersion, nil
}

// BuildDependencies runs `helm dependency update` on the local copy of the chart.
func (c *chart) UpdateDependencies() error {
	cmd := shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"dependency",
			"update",
			"--skip-refresh",
		},
		Dir: c.path,
	}
	return c.shellRunner.Run(cmd)
}

// PackageChart runs `helm package` to package a chart
func (c *chart) PackageChart(destPath string) error {
	cmd := shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"package",
			".",
			"--destination",
			destPath,
		},
		Dir: c.path,
	}
	return c.shellRunner.Run(cmd)
}

// GenerateDocs re-generates README documentation for the given chart
func (c *chart) GenerateDocs() error {
	cmd := shell.Command{
		Prog: helmDocsProg,
		Args: []string{
			".",
		},
		Dir: c.path,
	}
	return c.shellRunner.Run(cmd)
}

func (c *chart) LocalDependencies() []string {
	var dependencies []string

	for _, dependency := range c.manifest.Dependencies {
		log.Debug().Msgf("processing chart %s dependency: %v", c.name, dependency)
		if !strings.HasPrefix(dependency.Repository, fileRepoPrefix) {
			log.Debug().Msgf("dependency %s is not from a %s repository, ignoring", dependency.Name, fileRepoPrefix)
			continue
		}

		dependencies = append(dependencies, dependency.Name)
	}

	return dependencies
}

func (c *chart) SetDependencyVersion(dependencyName string, newVersion string) error {
	expression := fmt.Sprintf(`(.dependencies.[] | select(.name == %q) | .version) |= %q`, dependencyName, newVersion)
	manifestFile := path.Join(c.path, chartManifestFile)
	if err := yq.New(c.shellRunner).Write(expression, manifestFile); err != nil {
		return err
	}
	if err := c.reloadManifest(); err != nil {
		return err
	}
	for _, dependency := range c.manifest.Dependencies {
		if dependency.Name == dependencyName {
			if dependency.Version == newVersion {
				log.Debug().Msgf("updated version for dependency %s to %s in %s", dependencyName, newVersion, manifestFile)
				return nil
			} else {
				return fmt.Errorf("error setting dependency %s to version %s in %s: dependency not found", dependencyName, newVersion, manifestFile)
			}
		}
	}

	return fmt.Errorf("error setting dependency %s to version %s in %s: dependency not found", dependencyName, newVersion, manifestFile)
}

func (c *chart) ManifestVersion() string {
	return c.manifest.Version
}

func (c *chart) nextVersion(latestPublishedVersion string) string {
	sourceVersion := c.manifest.Version
	nextPublishedVersion, err := semver.MinorBump(latestPublishedVersion)

	if err != nil {
		log.Debug().Msgf("chart %s: could not determine next minor version for chart: %v", c.name, err)
		if !semver.IsValid(sourceVersion) {
			log.Debug().Msgf("chart %s: version in chart.yaml is invalid: %q", c.name, sourceVersion)
			log.Debug().Msgf("chart %s: falling back to default initial chart version: %q", c.name, initialChartVersion)
			return initialChartVersion
		}
		log.Debug().Msgf("chart %s: falling back to source version %q", c.name, sourceVersion)
		return sourceVersion
	}

	if !semver.IsValid(sourceVersion) {
		log.Debug().Msgf("chart %s: version in chart.yaml is invalid: %q", c.name, sourceVersion)
		log.Debug().Msgf("chart %s: will set to next computed version %q", c.name, nextPublishedVersion)
		return nextPublishedVersion
	}

	if semver.Compare(sourceVersion, nextPublishedVersion) > 0 {
		log.Debug().Msgf("chart %s: source version %q > next computed version %q, will use source version", c.name, sourceVersion, nextPublishedVersion)
		return sourceVersion
	}

	return nextPublishedVersion
}

func (c *chart) reloadManifest() error {
	manifestFile := path.Join(c.path, chartManifestFile)
	manifest, err := loadManifest(manifestFile)
	if err != nil {
		return fmt.Errorf("manifest reload failed: %v", err)
	}
	log.Debug().Msgf("chart %s: reloaded manifest file %s", c.name, manifestFile)
	c.manifest = manifest
	return nil
}

func loadManifest(manifestFile string) (ChartManifest, error) {
	var manifest ChartManifest

	content, err := ioutil.ReadFile(manifestFile)
	if err != nil {
		return manifest, fmt.Errorf("error reading chart manifest %s: %v", manifestFile, err)
	}

	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return manifest, fmt.Errorf("error parsing chart manifest %s: %v", manifestFile, err)
	}
	log.Debug().Msgf("loaded chart manifest from %s: %v", manifestFile, manifest)

	return manifest, nil
}
