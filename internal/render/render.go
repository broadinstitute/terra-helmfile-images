package render

import (
	"fmt"
	"os"
	"path"
	"terra-helmfile-tools/internal/render/cmd"
)

type Render struct {
	options *cmd.Options
	environments []string
}

func NewRender(options *cmd.Options) (*Render, error) {
	render := new(Render)
	render.options = options

	envDir := path.Join(options.ConfigRepoPath, "environments")
	if _, err := os.Stat(envDir); err != nil {
		return nil, fmt.Errorf("environments subdirectory does not exist in %s, is it a terra-helmfile clone?", options.ConfigRepoPath)
	}

	return render, nil
}

func (*Render) HelmUpdate() {

}

func (*Render) Render() {

}
