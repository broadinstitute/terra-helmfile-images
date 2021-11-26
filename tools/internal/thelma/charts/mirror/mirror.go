package mirror

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/publish"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/cli/views"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/tools/helm"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

// Mirror contains logic for mirror-hosting third-party Helm charts in a GCS Helm repository.
// Given a config file with a list of charts in public repositories, it will
// download the charts locally and upload them to a GCS bucket repository.
type Mirror interface {
	// ImportToMirror uploads configured charts to the GCS repository.
	// If a given chart version already exists in the repo, it won't be imported.
	// It returns a slice of ChartDefinitions representing the charts that were imported, if any.
	ImportToMirror() (imported []views.ChartRelease, err error)
}

// Implements Mirror interface
type mirror struct {
	publisher    publish.Publisher
	repositories []RepositoryDefinition
	charts       []ChartDefinition
	shellRunner  shell.Runner
}

// Struct used for deserializing repo definitions in mirror configuration
type RepositoryDefinition struct {
	Name string // Repository name, eg. "bitnami"
	Url  string // Repository url, eg. "https://charts.bitnami.com/bitnami"
}

// Struct used for deserializing chart definitions in mirror configuration
type ChartDefinition struct {
	Name      string // Name of the chart in form "<repo>/<name>", eg. "bitnami/mongodb"
	Version   string // Version of the chart, eg. "1.2.3"
	chartName string // chart component of Name, eg "mongodb"
	repoName  string // repository component of Name, eg "bitnami"
}

// Name of the repo. Eg. "terra-helm"
func (c ChartDefinition) RepoName() string {
	return c.repoName
}

// Name of the chart. Eg. "agora"
func (c ChartDefinition) ChartName() string {
	return c.chartName
}

// Struct for deserializing a mirror configuration file
type config struct {
	Repositories []RepositoryDefinition
	Charts       []ChartDefinition
}

func NewMirror(publisher publish.Publisher, shellRunner shell.Runner, configFile string) (Mirror, error) {
	m := &mirror{
		publisher:   publisher,
		shellRunner: shellRunner,
	}
	if err := m.loadConfig(configFile); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *mirror) ImportToMirror() ([]views.ChartRelease, error) {
	if len(m.charts) == 0 {
		log.Warn().Msgf("No charts defined in config file, won't upload any charts")
		return nil, nil
	}

	charts := m.chartsToUpload()

	if len(charts) == 0 {
		log.Info().Msgf("No new charts to upload, exiting")
		return nil, nil
	}

	if err := m.addHelmRepos(); err != nil {
		return nil, err
	}

	if err := m.fetchCharts(charts); err != nil {
		return nil, err
	}

	count, err := m.publisher.Publish()
	if err != nil {
		return nil, err
	}

	log.Info().Msgf("Imported %d new charts", count)
	return toView(charts), nil
}

func toView(chartDefns []ChartDefinition) []views.ChartRelease {
	var result []views.ChartRelease
	for _, chartDefn := range chartDefns {
		result = append(result, views.ChartRelease{
			Name:    chartDefn.ChartName(),
			Version: chartDefn.Version,
			Repo:    chartDefn.RepoName(),
		})
	}
	views.SortChartReleases(result)
	return result
}

func (m *mirror) chartsToUpload() []ChartDefinition {
	var result []ChartDefinition

	for _, chartDefn := range m.charts {
		if m.publisher.Index().HasVersion(chartDefn.chartName, chartDefn.Version) {
			log.Debug().Msgf("Repo index includes %s version %s, won't try to upload", chartDefn.chartName, chartDefn.Version)
			continue
		}

		log.Info().Msgf("Repo index does not include %s version %s, will upload it", chartDefn.chartName, chartDefn.Version)
		result = append(result, chartDefn)
	}

	return result
}

func (m *mirror) addHelmRepos() error {
	for _, repository := range m.repositories {
		if err := m.addHelmRepo(repository); err != nil {
			return err
		}
	}
	return nil
}

func (m *mirror) addHelmRepo(repository RepositoryDefinition) error {
	return m.shellRunner.Run(shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"repo",
			"add",
			repository.Name,
			repository.Url,
		},
	})
}

// Download charts with `helm fetch` into the publisher's chart directory
func (m *mirror) fetchCharts(charts []ChartDefinition) error {
	for _, chart := range charts {
		if err := m.fetchChart(chart); err != nil {
			return err
		}
	}

	return nil
}

// Download chart with `helm fetch` into the publisher's chart directory
func (m *mirror) fetchChart(chart ChartDefinition) error {
	return m.shellRunner.Run(shell.Command{
		Prog: helm.ProgName,
		Args: []string{
			"fetch",
			fmt.Sprintf(chart.Name),
			"--version",
			chart.Version,
			"--destination",
			m.publisher.ChartDir(),
		},
	})

}

func (m *mirror) loadConfig(configFile string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("error reading %s: %v", configFile, err)
	}
	cfg := &config{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("error parsing %s: %v", configFile, err)
	}

	repoMap := make(map[string]RepositoryDefinition)
	for _, repoDefn := range cfg.Repositories {
		_, exists := repoMap[repoDefn.Name]
		if exists {
			return fmt.Errorf("configuration error in %s: repository %s is defined more than once", configFile, repoDefn.Name)
		}
		repoMap[repoDefn.Name] = repoDefn
	}

	for i, chartDefn := range cfg.Charts {
		tokens := strings.Split(chartDefn.Name, "/")
		if len(tokens) != 2 {
			return fmt.Errorf(`"configuration error in %s: chart name must be a string of form <repository>/<chart> (eg. "bitnami/mongodb"), got %q`, configFile, chartDefn.Name)
		}

		chartDefn.repoName = tokens[0]
		chartDefn.chartName = tokens[1]
		cfg.Charts[i] = chartDefn

		if _, exists := repoMap[chartDefn.repoName]; !exists {
			return fmt.Errorf("configuration error in %s: chart %q references undefined repository %q", configFile, chartDefn.Name, chartDefn.repoName)
		}
	}

	m.repositories = cfg.Repositories
	m.charts = cfg.Charts

	return nil
}
