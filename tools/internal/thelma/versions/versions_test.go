package versions

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"testing"
)

const appDevVersionsContent = `
releases:
  agora:
    appVersion: ignored # Doesn't matter
    chartVersion: 0.0.0
`

const clusterAlphaVersionsContent = `
releases:
  prometheus:
    chartVersion: 0.1.2
`

func TestVersions_SetChartVersionIfDefined(t *testing.T) {
	thelmaHome := t.TempDir()
	mockRunner := shellmock.DefaultMockRunner()
	versions := New(thelmaHome, mockRunner)

	err := populateFakeVersionsDir(thelmaHome)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	//
	// Agora IS defined in versions/app/dev.yaml, so we should try to set the version.
	mockRunner.ExpectCmd(shell.Command{
		Prog: "yq",
		Args: []string{
			"eval",
			"--inplace",
			`.releases.agora.chartVersion = "1.2.3"`,
			path.Join(thelmaHome, "versions/app/dev.yaml"),
		},
	})

	err = versions.SetReleaseVersionIfDefined("agora", AppRelease, Dev, "1.2.3")
	assert.NoError(t, err)

	//
	// Prometheus IS defined in versions/cluster/alpha.yaml, so we should try to set the version.
	mockRunner.ExpectCmd(shell.Command{
		Prog: "yq",
		Args: []string{
			"eval",
			"--inplace",
			`.releases.prometheus.chartVersion = "4.5.6"`,
			path.Join(thelmaHome, "versions/cluster/alpha.yaml"),
		},
	})

	err = versions.SetReleaseVersionIfDefined("prometheus", ClusterRelease, Alpha, "4.5.6")
	assert.NoError(t, err)

	//
	// fakechart is NOT defined in versions/app/dev.yaml, so the version should NOT be set
	err = versions.SetReleaseVersionIfDefined("fakechart", AppRelease, Dev, "1.2.3")
	assert.NoError(t, err)

	mockRunner.AssertExpectations(t)
}

func TestReleaseType_String(t *testing.T) {
	assert.Equal(t, "app", AppRelease.String())
	assert.Equal(t, "cluster", ClusterRelease.String())
}

func TestReleaseType_UnmarshalYAML(t *testing.T) {
	var err error
	var r ReleaseType

	err = yaml.Unmarshal([]byte("app"), &r)
	assert.NoError(t, err)
	assert.Equal(t, AppRelease, r)

	err = yaml.Unmarshal([]byte("cluster"), &r)
	assert.NoError(t, err)
	assert.Equal(t, ClusterRelease, r)

	err = yaml.Unmarshal([]byte("invalid"), &r)
	assert.Error(t, err)
	assert.Regexp(t, "unknown release type", err)
}

func populateFakeVersionsDir(thelmaHome string) error {
	appDevVersionsFile := path.Join(thelmaHome, versionsDir, "app", "dev.yaml")
	clusterAlphaVersionsFile := path.Join(thelmaHome, versionsDir, "cluster", "alpha.yaml")

	if err := writeFakeVersionsFile(appDevVersionsFile, appDevVersionsContent); err != nil {
		return err
	}
	if err := writeFakeVersionsFile(clusterAlphaVersionsFile, clusterAlphaVersionsContent); err != nil {
		return err
	}

	return nil
}

func writeFakeVersionsFile(fileName string, content string) error {
	if err := os.MkdirAll(path.Dir(fileName), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
		return err
	}
	return nil
}