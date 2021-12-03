package resolver

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/rs/zerolog/log"
)

// Mode is an enum type referring to the two types of releases supported by terra-helmfile.
type Mode int

const (
	Development Mode = iota // Prefer source copies of charts
	Deploy                  // Prefer released versions of charts (in versions/ directories)
)

type Options struct {
	Mode      Mode   // development / deploy
	SourceDir string // path to chart source directory
	CacheDir  string // path where downloaded charts should be cached
}

type ChartResolver interface {
	// Depending on resolution mode, either download and unpack the chart or run "helm dependency update" on the source
	Resolve(chart ChartRelease) (string, error)
}

type chartResolver struct {
	options Options
	updater Updater
	cache   ChartCache
}

func NewChartResolver(runner shell.Runner, options Options) ChartResolver {
	_updater := NewUpdater(options.SourceDir, runner)
	_cache := NewChartCache(options.CacheDir, runner)
	return &chartResolver{
		options: options,
		updater: _updater,
		cache:   _cache,
	}
}

func (r *chartResolver) Resolve(chart ChartRelease) (string, error) {
	existsInSource, err := r.updater.ChartExists(chart.Name)
	if err != nil {
		return "", err
	}

	if r.options.Mode == Development {
		// In development mode, render from source (unless the chart does not exist in source)
		if !existsInSource {
			log.Warn().Msgf("Chart %s does not exist in source dir %s, will try to download from Helm repo", chart.Name, r.options.SourceDir)
			return r.cache.Fetch(chart)
		}
		return r.updater.UpdateIfNeeded(chart.Name)
	}

	// We're in deploy mode, so download released version from Helm repo
	cachePath, err := r.cache.Fetch(chart)
	if err != nil {
		if !existsInSource {
			return "", err
		}
		sourceVersion, versionErr := r.updater.SourceVersion(chart.Name)
		if versionErr != nil {
			log.Warn().Msgf("error checking source version for %s: %v", chart.Name, versionErr)
			return "", err
		}
		if sourceVersion == chart.Version {
			log.Warn().Msgf("Failed to download chart %s/%s version %s from Helm repo, will fall back to source copy", chart.Repo, chart.Name, chart.Version)
			return r.updater.UpdateIfNeeded(chart.Name)
		}
		return "", err
	}

	return cachePath, nil
}
