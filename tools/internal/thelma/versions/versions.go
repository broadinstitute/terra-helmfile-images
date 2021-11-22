package versions

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"path"
)

const versionsDir = "versions"

// Versions is for manipulating chart release versions in terra-helmfile (eg. versions/app/dev.yaml, versions/cluster/dev.yaml)
type Versions interface {
	// LoadSnapshot returns a Snapshot reference for the given release type & version set
	LoadSnapshot(releaseType ReleaseType, versionSet Set) (Snapshot, error)
}

// Implements public Versions interface
type versions struct {
	thelmaHome string
	shellRunner shell.Runner
}

// NewVersions returns a new Versions instance
func NewVersions(thelmaHome string, shellRunner shell.Runner) Versions {
	return &versions{
		thelmaHome: thelmaHome,
		shellRunner: shellRunner,
	}
}

func (v *versions) LoadSnapshot(releaseType ReleaseType, versionSet Set) (Snapshot, error) {
	filePath := snapshotPath(v.thelmaHome, releaseType, versionSet)
	return loadSnapshot(filePath, v.shellRunner)
}

// Returns filePath to snapshot file release type / version set. (eg "versions/app/dev.yaml")
func snapshotPath(thelmaHome string, releaseType ReleaseType, set Set) string {
	fileName := fmt.Sprintf("%s.yaml", set)
	return path.Join(thelmaHome, versionsDir, releaseType.String(), fileName)
}

