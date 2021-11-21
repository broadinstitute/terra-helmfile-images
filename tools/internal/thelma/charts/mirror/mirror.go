package mirror

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/publish"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/helm"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
)

// Mirror contains logic for mirror-hosting third-party Helm charts in a GCS Helm repository.
// Given a config file with a list of charts in public repositories, it will
// download the charts locally and upload them to a GCS bucket repository.
type Mirror interface {
	// ImportToMirror uploads configured charts to the GCS repository.
	// If commit is false, the import is a "dry run" (no charts are uploaded)
	ImportToMirror() error
}

// Implements Mirror interface
type mirror struct {
	publisher publish.Publisher
	repositories []RepoDefinition
	charts []ChartDefinition
	shellRunner shell.Runner
}

// Used for deserializing repo definitions in mirror configuration
type RepoDefinition struct {
	Name string
	Url string
}

// Used for deserializing chart definitions in mirror configuration
type ChartDefinition struct {
	Name string
	Repo string
	Version string
}

// Struct for deserializing a mirror configuration file
type config struct {
	Repositories []RepoDefinition
	Charts []ChartDefinition
}

func NewMirror(publisher publish.Publisher, shellRunner shell.Runner, configFile string) (Mirror, error) {
	m := &mirror{
		publisher: publisher,
		shellRunner: shellRunner,
	}
	if err := m.loadConfig(configFile); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *mirror) ImportToMirror() error {
	if len(m.charts) == 0 {
		log.Warn().Msgf("No charts defined in config file, won't upload any charts")
		return nil
	}

	charts := m.chartsToUpload()

	if len(charts) == 0 {
		log.Info().Msgf("No new charts to upload, exiting")
		return nil
	}

	if err := m.addHelmRepos(); err != nil {
		return err
	}

	if err := m.fetchCharts(charts); err != nil {
		return err
	}

	count, err := m.publisher.Publish()
	if err != nil {
		return err
	}

	log.Info().Msgf("Imported %d new charts", count)
	return nil
}

func (m *mirror) chartsToUpload() []ChartDefinition {
	var result []ChartDefinition

	for _, chartDefn := range m.charts {
		if m.publisher.Index().HasVersion(chartDefn.Name, chartDefn.Version) {
			log.Debug().Msgf("Repo index includes %s version %s, won't try to upload", chartDefn.Name, chartDefn.Version)
			continue
		}

		log.Info().Msgf("Repo index does include %s version %s, will upload it", chartDefn.Name, chartDefn.Version)
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

func (m *mirror) addHelmRepo(repository RepoDefinition) error {
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
			fmt.Sprintf("%s/%s", chart.Repo, chart.Name),
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

	repoMap := make(map[string]RepoDefinition)
	for _, repoDefn := range cfg.Repositories {
		_, exists := repoMap[repoDefn.Name]
		if exists {
			return fmt.Errorf("configuration error in %s: repository %s is defined more than once", configFile, repoDefn.Name)
		}
		repoMap[repoDefn.Name] = repoDefn
	}

	for _, chartDefn := range cfg.Charts {
		if _, exists := repoMap[chartDefn.Repo]; !exists {
			return fmt.Errorf("configuration error in %s: chart %s references undefined repository %s", configFile, chartDefn.Name, chartDefn.Repo)
		}
	}

	m.repositories = cfg.Repositories
	m.charts = cfg.Charts

	return nil
}