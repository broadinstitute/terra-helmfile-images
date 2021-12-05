package resolver

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"strings"
)

type ResolutionType int

const (
	Local  ResolutionType = iota // Indicates matching chart was found on local filesystem
	Remote                       // Indicates matching chart was found in Helm repo
)

func (t ResolutionType) String() string {
	if t.IsLocal() {
		return "local"
	} else {
		return "repo"
	}
}

func (t ResolutionType) IsLocal() bool {
	switch t {
	case Local:
		return true
	case Remote:
		return false
	default:
		panic(fmt.Errorf("unknown resolution type: %d", t))
	}
}

// ResolvedChart represents the outcome of a successful resolution of a chart release.
type ResolvedChart interface {
	// Returns path where the resolved chart can be found on disk
	Path() string
	// Returns version of the resolved chart.
	Version() string
	// Returns a succinct description of where the chart was found.
	// If local, this will be path on disk relative to current working directory. (eg. "./charts/agora")
	// If repo, this will be the name of the Helm repo. (eg. "terra-helm")
	SourceDescription() string
}

func NewResolvedChart(path string, chartVersion string, _type ResolutionType, release ChartRelease) ResolvedChart {
	return &resolvedChart{
		path:           path,
		chartVersion:   chartVersion,
		resolutionType: _type,
		chartRelease:   release,
	}
}

type resolvedChart struct {
	path           string
	chartVersion   string
	resolutionType ResolutionType
	chartRelease   ChartRelease
}

func (r *resolvedChart) Path() string {
	return r.path
}

func (r *resolvedChart) Version() string {
	return r.chartVersion
}

func (r *resolvedChart) SourceDescription() string {
	if r.resolutionType.IsLocal() {
		cwd, err := os.Getwd()
		if err != nil {
			log.Warn().Msgf("resolver: unexpected error calling os.GetWd(): %v", err)
			return r.Path()
		}
		abs, err := filepath.Abs(r.Path())
		if err != nil {
			log.Warn().Msgf("resolver: unexpected error calling filepath.Abs(): %v", err)
			return r.Path()
		}
		relPath, err := filepath.Rel(cwd, abs)
		if err != nil {
			log.Warn().Msgf("resolver: unexpected error calling filepath.Rel(): %v", err)
			return r.Path()
		}

		// Need to use strings.Join instead of path.Join because the latter omits "."
		return strings.Join([]string{".", relPath}, string(os.PathSeparator))
	} else {
		return r.chartRelease.Repo
	}
}
