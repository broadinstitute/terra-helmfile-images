package gitops

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/semver"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/tools/yq"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
)

type VersionSnapshot interface {
	// ReleaseDefined returns true if the given release is defined in this snapshot
	ReleaseDefined(releaseName string) bool
	// ChartVersion returns the chartVersion for the given release in this snapshot. If the release is not defined, returns ""
	ChartVersion(releaseName string) string
	// AppVersion returns the chartVersion for the given release in this snapshot. If the release is not defined, returns ""
	AppVersion(releaseName string) string
	// UpdateChartVersionIfDefined sets the chartVersion for the given release to the given version.
	// If the release is not defined, or if newVersion <= the current chart version, this function does nothing.
	UpdateChartVersionIfDefined(releaseName string, newVersion string) error
}

// Represents a version snapshot file in terra-helmfile (eg. "versions/app/dev.yaml")
type snapshot struct {
	filePath string
	data     *snapshotData
	yq       yq.Yq
}

// struct used for deserializing a version snapshot
type snapshotData struct {
	Releases map[string]struct {
		ChartVersion string `yaml:"chartVersion"`
		AppVersion   string `yaml:"appVersion"`
	} `yaml:"releases"`
}

func loadSnapshot(filePath string, shellRunner shell.Runner) (VersionSnapshot, error) {
	data, err := readSnapshotFile(filePath)
	if err != nil {
		return nil, err
	}

	return &snapshot{
		filePath: filePath,
		data:     data,
		yq:       yq.New(shellRunner),
	}, nil
}

func (s *snapshot) ReleaseDefined(releaseName string) bool {
	_, exists := s.data.Releases[releaseName]
	return exists
}

func (s *snapshot) ChartVersion(releaseName string) string {
	if !s.ReleaseDefined(releaseName) {
		return ""
	}
	return s.data.Releases[releaseName].ChartVersion
}

func (s *snapshot) AppVersion(releaseName string) string {
	if !s.ReleaseDefined(releaseName) {
		return ""
	}
	return s.data.Releases[releaseName].AppVersion
}

func (s *snapshot) UpdateChartVersionIfDefined(releaseName string, newVersion string) error {
	defined := s.ReleaseDefined(releaseName)
	if !defined {
		log.Warn().Msgf("Won't update chart version for release %s in %s (release not found in file)", releaseName, s.filePath)
		return nil
	}

	currentVersion := s.ChartVersion(releaseName)
	if semver.IsValid(currentVersion) {
		if semver.Compare(newVersion, currentVersion) <= 0 {
			log.Warn().Msgf("Won't update chart version for release %s in %s (new chart version %q <= current version %q)", releaseName, s.filePath, newVersion, currentVersion)
			return nil
		}
	} else {
		log.Warn().Msgf("Current chart version %q for release %s in %s is invalid", currentVersion, releaseName, s.filePath)
	}

	if err := s.setReleaseVersion(releaseName, newVersion); err != nil {
		return fmt.Errorf("error updating version snapshot %s: %v", s.filePath, err)
	}

	if err := s.reload(); err != nil {
		return fmt.Errorf("error updating version snapshot %s: %v", s.filePath, err)
	}
	if !s.ReleaseDefined(releaseName) {
		return fmt.Errorf("error updating version snapshot %s: malformed after updating %s chart version", s.filePath, releaseName)
	}

	oldVersion := currentVersion
	updatedVersion := s.ChartVersion(releaseName)
	if updatedVersion != newVersion {
		return fmt.Errorf("error updating version snapshot %s: chart version incorrect after updating %s chart version (should be %q, is %q)", s.filePath, releaseName, newVersion, updatedVersion)
	}

	log.Info().Msgf("Set chart version for release %s to %s in %s (was %q)", releaseName, newVersion, s.filePath, oldVersion)
	return nil
}

// Sets the chart version for the given release
func (s *snapshot) setReleaseVersion(releaseName string, newVersion string) error {
	expression := fmt.Sprintf(".releases.%s.chartVersion = %q", releaseName, newVersion)
	return s.yq.Write(expression, s.filePath)
}

// Reload snapshot data from disk
func (s *snapshot) reload() error {
	data, err := readSnapshotFile(s.filePath)
	if err != nil {
		return fmt.Errorf("error reloading snapshot: %v", err)
	}
	s.data = data
	return nil
}

// Unmarshal the snapshot at the given filePath into a struct
func readSnapshotFile(filePath string) (*snapshotData, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file %s: %v", filePath, err)
	}

	snapshot := &snapshotData{}
	if err := yaml.Unmarshal(content, snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot file %s: %v", filePath, err)
	}

	if snapshot.Releases == nil {
		return nil, fmt.Errorf("empty snapshot file %s: %v", filePath, err)
	}

	return snapshot, nil
}
