package mirror

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shellmock"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/publish"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo/index"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/tools/helm"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUploadToMirror(t *testing.T) {
	chartDir := t.TempDir()

	publisher := publish.NewMockPublisher()
	_index := index.NewMockIndex()
	runner := shellmock.DefaultMockRunner()

	_index.On("HasVersion", "mongodb", "1.2.3").Return(false, nil)
	_index.On("HasVersion", "mongodb", "0.1.0").Return(true, nil)
	_index.On("HasVersion", "reloader", "4.0.2").Return(true, nil)
	publisher.On("Index").Return(_index)
	publisher.On("ChartDir").Return(chartDir)
	publisher.On("Publish").Return(1, nil)

	runner.ExpectCmd(shell.Command{
		Prog: helm.ProgName,
		Args: []string{"repo", "add", "bitnami", "https://bitnami.com/charts"},
	})

	runner.ExpectCmd(shell.Command{
		Prog: helm.ProgName,
		Args: []string{"repo", "add", "stakater", "https://stakater.com/charts"},
	})

	runner.ExpectCmd(shell.Command{
		Prog: helm.ProgName,
		Args: []string{"fetch", "bitnami/mongodb", "--version", "1.2.3", "--destination", chartDir},
	})

	mirr, err := NewMirror(publisher, runner, "testdata/config.yaml")
	assert.NoError(t, err)

	err = mirr.ImportToMirror()
	assert.NoError(t, err)

	runner.AssertExpectations(t)
	publisher.AssertExpectations(t)
	_index.AssertExpectations(t)
}
