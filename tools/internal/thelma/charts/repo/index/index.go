package index

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/semver"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"sort"
)

type Entry struct {
	Version string `yaml:"version"`
}
type Index struct {
	Entries map[string][]Entry `yaml:"entries"`
}

func LoadFromFile(filePath string) (*Index, error) {
	indexContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading index file %s: %v", filePath, err)
	}

	var index Index
	if err := yaml.Unmarshal(indexContent, &index); err != nil {
		return nil, fmt.Errorf("error parsing index file %s: %v", filePath, err)
	}

	return &index, nil
}

func (index *Index) LatestVersion(chartName string) string {
	if index.Entries == nil || len(index.Entries) == 0 {
		log.Warn().Msgf("index is empty, can't look up chart version for %s", chartName)
		return ""
	}

	entries, exists := index.Entries[chartName]
	if !exists {
		log.Debug().Msgf("index does not have an entry for chart %s", chartName)
		return ""
	}

	var versions []string
	for _, entry := range entries {
		if !semver.IsValid(entry.Version) {
			log.Warn().Msgf("index has invalid semver %q for chart %s, ignoring", entry.Version, chartName)
			continue
		}

		versions = append(versions, entry.Version)
	}

	if len(versions) == 0 {
		return ""
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(versions[i], versions[j]) < 0
	})

	return versions[len(versions)-1]
}