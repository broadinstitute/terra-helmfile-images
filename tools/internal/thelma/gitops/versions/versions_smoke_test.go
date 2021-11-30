//go:build smoke
// +build smoke

package versions

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops/release"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSnapshot_UpdateChartVersionIfDefined_Smoke(t *testing.T) {
	thelmaHome := t.TempDir()
	runner := shell.NewDefaultRunner()
	_versions := NewVersions(thelmaHome, runner).(*versions)

	var err error

	err = initializeFakeVersionsDir(thelmaHome)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// load the snapshot
	_snapshot, err := _versions.LoadSnapshot(release.AppType, Dev)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.True(t, _snapshot.ReleaseDefined("agora"))
	assert.Equal(t, "0.0.0", _snapshot.ChartVersion("agora"))

	// set the chart version
	err = _snapshot.UpdateChartVersionIfDefined("agora", "7.8.9")
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// verify the version was updated
	assert.True(t, _snapshot.ReleaseDefined("agora"))
	assert.Equal(t, "7.8.9", _snapshot.ChartVersion("agora"))
}
