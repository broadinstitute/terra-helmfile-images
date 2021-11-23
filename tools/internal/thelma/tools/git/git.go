package git

import "github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"

const progName = "git"

type Clone interface {
	// GetUpdatedFiles returns the list of files in the repo that were updated by a given commit
	GetUpdatedFiles(ref string) ([]string, error)
}

// Implements the Clone interface
type clone struct {
	clonePath string
	shellRunner shell.Runner
}

func NewClone(clonePath string, shellRunner shell.Runner) Clone {
	return &clone{
		clonePath: clonePath,
		shellRunner: shellRunner,
	}
}

func (c clone) GetUpdatedFiles(ref string) ([]string, error) {
	_ = progName
	panic("TODO")
}
