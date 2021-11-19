//go:build smoke
// +build smoke

package versions

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVersions_Smoke(t *testing.T) {
	thelmaHome := t.TempDir()
	runner := shell.NewRealRunner()
	versions := New(thelmaHome, runner).(*versions)

	var err error

	err = populateFakeVersionsDir(thelmaHome)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// set the chart version
	err = versions.SetReleaseVersionIfDefined("agora", AppRelease, Dev, "7.8.9")
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// verify the version was actually set
	snapshot, err := versions.readVersionsSnapshot(AppRelease, Dev)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	assert.NoError(t, err)
	assert.Equal(t, "7.8.9", snapshot.Releases["agora"].ChartVersion)
}

