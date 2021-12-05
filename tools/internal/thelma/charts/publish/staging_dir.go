package publish

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo"
	"path"
)

const prevIndexFile = "index-prev.yaml"
const newIndexFile = "index.yaml"

type stagingDir struct {
	root string
}

func (d *stagingDir) chartDir() string {
	return path.Join(d.root, repo.ChartDir)
}

func (d *stagingDir) prevIndexFile() string {
	return path.Join(d.root, prevIndexFile)
}

func (d *stagingDir) newIndexFile() string {
	return path.Join(d.root, newIndexFile)
}
