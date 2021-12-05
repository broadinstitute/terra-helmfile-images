package index

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/semver"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"sort"
)

// Index represents a Helm repo's index.yaml
type Index interface {
	// Versions returns a list of all published versions of the chart
	Versions(chartName string) []string
	// HasVersion returns true if the index contains the given version for the given chart
	HasVersion(chartName string, version string) bool
	// MostRecentVersion returns the most recent / highest semantic version of the chart in the index
	MostRecentVersion(chartName string) string
}

// Entry is for deserializing chart entries in a Helm repo index.yaml
type Entry struct {
	Version string `yaml:"version"`
}

// index implements the Index interface
type index struct {
	Entries map[string][]Entry `yaml:"entries"`
}

// LoadFromFile parses an index from a file
func LoadFromFile(filePath string) (*index, error) {
	indexContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading index file %s: %v", filePath, err)
	}

	var _index index
	if err := yaml.Unmarshal(indexContent, &_index); err != nil {
		return nil, fmt.Errorf("error parsing index file %s: %v", filePath, err)
	}

	return &_index, nil
}

// HasVersion returns true if the index contains the given version for the given chart
func (index *index) HasVersion(chartName string, version string) bool {
	for _, publishedVersion := range index.Versions(chartName) {
		if version == publishedVersion {
			return true
		}
	}

	return false
}

// Versions returns a list of all published versions of the chart
func (index *index) Versions(chartName string) []string {
	var versions []string

	if index.Entries == nil || len(index.Entries) == 0 {
		log.Warn().Msgf("index is empty, can't look up chart version for %s", chartName)
		return versions
	}

	entries, exists := index.Entries[chartName]
	if !exists {
		log.Debug().Msgf("index does not have an entry for chart %s", chartName)
		return versions
	}

	for _, entry := range entries {
		if !semver.IsValid(entry.Version) {
			log.Warn().Msgf("index has invalid semver %q for chart %s, ignoring", entry.Version, chartName)
			continue
		}

		versions = append(versions, entry.Version)
	}

	return versions
}

// MostRecentVersion returns the most recent / highest semantic version of the chart in the index
func (index *index) MostRecentVersion(chartName string) string {
	versions := index.Versions(chartName)

	if len(versions) == 0 {
		return ""
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(versions[i], versions[j]) < 0
	})

	return versions[len(versions)-1]
}
