//go:build smoke
// +build smoke

package gitops

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSnapshot_UpdateChartVersionIfDefined_Smoke(t *testing.T) {
	thelmaHome := t.TempDir()
	runner := shell.NewDefaultRunner()

	var err error

	err = initializeFakeVersionsDir(thelmaHome)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	v, err := NewVersions(thelmaHome, runner)
	if !assert.NoError(t, err) {
		t.Fail()
	}

	_versions := v.(*versions)

	// load the snapshot
	_snapshot := _versions.GetSnapshot(AppReleaseType, Dev)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.True(t, _snapshot.ReleaseDefined("agora"))
	assert.Equal(t, "0.10.0", _snapshot.ChartVersion("agora"))

	// set the chart version
	err = _snapshot.UpdateChartVersionIfDefined("agora", "7.8.9")
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// verify the version was updated
	assert.True(t, _snapshot.ReleaseDefined("agora"))
	assert.Equal(t, "7.8.9", _snapshot.ChartVersion("agora"))
}
