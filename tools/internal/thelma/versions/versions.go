package versions

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/yq"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path"
)

const versionsDir = "versions"

// Versions is for manipulating chart release versions in terra-helmfile (eg. versions/app/dev.yaml, versions/cluster/dev.yaml)
type Versions interface {
	// SetReleaseVersionIfDefined sets the chartVersion for the given release.
	SetReleaseVersionIfDefined(releaseName string, releaseType ReleaseType, versionSet Set, newVersion string) error
}

// Implements public Versions interface
type versions struct {
	thelmaHome string
	yq         yq.Yq
}

// Used for parsing version snapshot files (like "versions/app/dev.yaml")
type versionsSnapshot struct {
	Releases map[string]struct {
		ChartVersion string `yaml:"chartVersion"`
		AppVersion   string `yaml:"appVersion"`
	} `yaml:"releases"`
}

// New returns a new Versions instance
func New(thelmaHome string, shellRunner shell.Runner) Versions {
	return &versions{
		thelmaHome: thelmaHome,
		yq:         yq.New(shellRunner),
	}
}

func (v *versions) SetReleaseVersionIfDefined(releaseName string, releaseType ReleaseType, versionSet Set, newVersion string) error {
	defined, err := v.releaseDefined(releaseName, releaseType, versionSet)
	if err != nil {
		return fmt.Errorf("unexpected error updating versions file for %s: %v", releaseName, err)
	}
	if !defined {
		log.Warn().Msgf("No configured version for release %s found in %s, won't update", releaseName, v.versionsSnapshotPath(releaseType, versionSet))
		return nil
	}

	if err := v.setReleaseVersion(releaseName, releaseType, versionSet, newVersion); err != nil {
		return err
	}

	// Verify the version was set correctly
	snapPath := v.versionsSnapshotPath(releaseType, versionSet)
	versionSnap, err := v.readVersionsSnapshot(releaseType, versionSet)
	if err != nil {
		return fmt.Errorf("error updating version snapshot %s: %v", snapPath, err)
	}
	release, exists := versionSnap.Releases[releaseName]
	if !exists {
		snapPath := v.versionsSnapshotPath(releaseType, versionSet)
		return fmt.Errorf("error updating version snapshot %s: malformed after updating %s chart version", snapPath, releaseName)
	}
	if release.ChartVersion != newVersion {
		return fmt.Errorf("error updating version snapshot %s: chart version incorrect after updating %s chart version (should be %q, is %q)", snapPath, releaseName, newVersion, release.ChartVersion)
	}
	return nil
}

// Returns true if this release is defined in the target version file
func (v *versions) releaseDefined(releaseName string, releaseType ReleaseType, versionSet Set) (bool, error) {
	versionSnap, err := v.readVersionsSnapshot(releaseType, versionSet)
	if err != nil {
		return false, err
	}
	_, exists := versionSnap.Releases[releaseName]
	return exists, nil
}

// Sets release version in the target version file to the given version
func (v *versions) setReleaseVersion(releaseName string, releaseType ReleaseType, versionSet Set, version string) error {
	targetFile := v.versionsSnapshotPath(releaseType, versionSet)
	expression := fmt.Sprintf(".releases.%s.chartVersion = %q", releaseName, version)
	return v.yq.Write(expression, targetFile)
}

// Unmarshal the given versions file into a struct
func (v *versions) readVersionsSnapshot(releaseType ReleaseType, versionSet Set) (*versionsSnapshot, error) {
	targetFile := v.versionsSnapshotPath(releaseType, versionSet)
	data, err := os.ReadFile(targetFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read version file %s: %v", targetFile, err)
	}

	snapshot := &versionsSnapshot{}
	if err := yaml.Unmarshal(data, snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse version file %s: %v", targetFile, err)
	}

	if snapshot.Releases == nil {
		return nil, fmt.Errorf("empty version file %s: %v", targetFile, err)
	}

	return snapshot, nil
}

// Returns version file for given release type / version set. (eg "versions/app/dev.yaml")
func (v *versions) versionsSnapshotPath(releaseType ReleaseType, versionSet Set) string {
	fileName := fmt.Sprintf("%s.yaml", versionSet)
	return path.Join(v.thelmaHome, versionsDir, releaseType.String(), fileName)
}
