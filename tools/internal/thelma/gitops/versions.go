package gitops

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"path"
)

const versionsDir = "versions"

// Versions is for manipulating chart release versions in terra-helmfile (eg. versions/app/dev.yaml, versions/cluster/dev.yaml)
type Versions interface {
	// GetSnapshot returns a VersionSnapshot reference for the given release type & version set
	GetSnapshot(releaseType ReleaseType, versionSet VersionSet) VersionSnapshot
}

// Implements public Versions interface
type versions struct {
	thelmaHome  string
	shellRunner shell.Runner
	snapshots   map[ReleaseType]map[VersionSet]VersionSnapshot
}

// NewVersions returns a new Versions instance
func NewVersions(thelmaHome string, shellRunner shell.Runner) (Versions, error) {
	snapshots, err := loadSnapshots(thelmaHome, shellRunner)
	if err != nil {
		return nil, err
	}

	return &versions{
		thelmaHome:  thelmaHome,
		shellRunner: shellRunner,
		snapshots:   snapshots,
	}, nil
}

func (v *versions) GetSnapshot(releaseType ReleaseType, versionSet VersionSet) VersionSnapshot {
	return v.snapshots[releaseType][versionSet]
}

// Returns filePath to snapshot file release type / version set. (eg "versions/app/dev.yaml")
func snapshotPath(thelmaHome string, releaseType ReleaseType, set VersionSet) string {
	fileName := fmt.Sprintf("%s.yaml", set)
	return path.Join(thelmaHome, versionsDir, releaseType.String(), fileName)
}

func loadSnapshots(thelmaHome string, shellRunner shell.Runner) (map[ReleaseType]map[VersionSet]VersionSnapshot, error) {
	snapshots := make(map[ReleaseType]map[VersionSet]VersionSnapshot)

	for _, releaseType := range ReleaseTypes() {
		_, exists := snapshots[releaseType]
		if !exists {
			snapshots[releaseType] = make(map[VersionSet]VersionSnapshot)
		}

		for _, versionSet := range VersionSets() {
			fileName := snapshotPath(thelmaHome, releaseType, versionSet)
			snap, err := loadSnapshot(fileName, shellRunner)
			if err != nil {
				return nil, fmt.Errorf("error loading %s %s snapshot from %s: %v", releaseType, versionSet, fileName, err)
			}
			snapshots[releaseType][versionSet] = snap
		}
	}

	return snapshots, nil
}
