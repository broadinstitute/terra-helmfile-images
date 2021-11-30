package source

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops/release"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops/versions"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path"
)

const configFile = ".autorelease.yaml"
const targetVersionSet = versions.Dev

// AutoReleaser bumps chart versions in versions/app/dev.yaml & friends when a new chart version is released
type AutoReleaser interface {
	// UpdateReleaseVersion updates the version file
	UpdateReleaseVersion(chart Chart, version string) error
}

// Struct for parsing an autorelease.yaml config file
type config struct {
	Enabled bool `yaml:"enabled"` // whether updates to this chart should be added to release train. defaults to true
	Release struct {
		Name string              `yaml:"name"` // name of the "release", defaults to chart name
		Type release.ReleaseType `yaml:"type"` // either "app" or "cluster", defaults to app
	} `yaml:"release"`
}

// Implements the public AutoReleaser interface
type autoReleaser struct {
	versions versions.Versions
}

func NewAutoReleaser(versions versions.Versions) AutoReleaser {
	return &autoReleaser{
		versions: versions,
	}
}

func (a *autoReleaser) UpdateReleaseVersion(chart Chart, newVersion string) error {
	cfg := loadConfig(chart)
	if !cfg.Enabled {
		return nil
	}

	snapshot, err := a.versions.LoadSnapshot(cfg.Release.Type, targetVersionSet)
	if err != nil {
		return err
	}
	return snapshot.UpdateChartVersionIfDefined(cfg.Release.Name, newVersion)
}

// load .autorelease.yaml config file from chart source directory if it exists
func loadConfig(chart Chart) config {
	cfg := config{}

	// Set defaults
	cfg.Enabled = true
	cfg.Release.Name = chart.Name()
	cfg.Release.Type = release.AppType

	file := path.Join(chart.Path(), configFile)
	_, err := os.Stat(file)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn().Msgf("unexpected error reading %s: %v, falling back to default config", file, err)
		}
		// no config file or can't read it, so return empty
		return cfg
	}

	data, err := os.ReadFile(file)
	if err != nil {
		log.Warn().Msgf("unexpected error reading %s: %v, falling back to default config", file, err)
		return cfg
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Warn().Msgf("unexpected error parsing %s: %v, falling back to default config", file, err)
		return cfg
	}

	return cfg
}
