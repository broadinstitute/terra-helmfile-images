package source

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/semver"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/helm"
	"github.com/rs/zerolog/log"
)

const initialChartVersion = "0.1.0"
const yqProg = "yq"
const helmDocsProg = "helm-docs"

// Struct representing a chart source directory on the local filesystem.
type Chart struct {
	name        string        // name of the chart
	path        string        // path to the chart directory on the local filesystem
	manifest    ChartManifest // manifest parsed subset of Chart.yaml
	shellRunner shell.Runner  // shell runner instance to use for executing commands
}

func (chart Chart) Name() string {
	return chart.name
}

// Update Chart version in Chart.yaml
func (chart Chart) BumpChartVersion(latestPublishedVersion string) error {
	nextVersion := chart.nextVersion(latestPublishedVersion)

	cmd := shell.Command{
		Prog: yqProg,
		Args: []string{
			"e",
			"-i",
			fmt.Sprintf(".version = %q", nextVersion),
			chartManifestFile,
		},
		Dir: chart.path,
	}

	return chart.shellRunner.Run(cmd)
}

// BuildDependencies runs `helm dependency build` on the local copy of the chart.
func (chart Chart) BuildDependencies() error {
	cmd := shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"dependency",
			"build",
			"--skip-refresh",
		},
		Dir: chart.path,
	}
	return chart.shellRunner.Run(cmd)
}

// PackageChart runs `helm package` to package a chart
func (chart Chart) PackageChart(destPath string) error {
	cmd := shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"package",
			".",
			"--destination",
			destPath,
		},
		Dir: chart.path,
	}
	return chart.shellRunner.Run(cmd)
}

// GenerateDocs re-generates README documentation for the given chart
func (chart Chart) GenerateDocs() error {
	cmd := shell.Command{
		Prog: helmDocsProg,
		Args: []string{
			".",
		},
		Dir: chart.path,
	}
	return chart.shellRunner.Run(cmd)
}

func (chart Chart) nextVersion(latestPublishedVersion string) string {
	sourceVersion := chart.manifest.Version
	nextPublishedVersion, err := semver.MinorBump(latestPublishedVersion)

	if err != nil {
		log.Debug().Msgf("chart %s: could not determine next minor version for chart: %v", chart.name, err)
		if !semver.IsValid(sourceVersion) {
			log.Debug().Msgf("chart %s: version in Chart.yaml is invalid: %q", chart.name, sourceVersion)
			log.Debug().Msgf("chart %s: falling back to default initial chart version: %q", chart.name, initialChartVersion)
			return initialChartVersion
		}
		log.Debug().Msgf("chart %s: falling back to source version %q", chart.name, sourceVersion)
		return sourceVersion
	}

	if !semver.IsValid(sourceVersion) {
		log.Debug().Msgf("chart %s: version in Chart.yaml is invalid: %q", chart.name, sourceVersion)
		log.Debug().Msgf("chart %s: will set to next computed version %q", chart.name, nextPublishedVersion)
		return nextPublishedVersion
	}

	if semver.Compare(sourceVersion, nextPublishedVersion) > 0 {
		log.Debug().Msgf("chart %s: source version %q > next computed version %q, will use source version", chart.name, sourceVersion, nextPublishedVersion)
		return sourceVersion
	}

	return nextPublishedVersion
}
