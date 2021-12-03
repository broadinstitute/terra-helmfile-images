package yq

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
)

const prog = "yq"

// Yq is for running `yq` commands.
// It's useful for updating YAML files without stripping comments or formatting.
type Yq interface {
	// Write updates a file based on a yq expression. I.e. it runs `yq eval --inplace <expression> <file>`
	Write(expression string, targetFile string) error
}

type yq struct {
	shellRunner shell.Runner
}

func New(runner shell.Runner) Yq {
	return &yq{
		shellRunner: runner,
	}
}

func (y *yq) Write(expression string, targetFile string) error {
	return y.shellRunner.Run(shell.Command{
		Prog: prog,
		Args: []string{
			"eval",
			"--inplace",
			expression,
			targetFile,
		},
	})
}
