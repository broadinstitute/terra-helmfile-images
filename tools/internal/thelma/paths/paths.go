package paths

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/config"
	"github.com/rs/zerolog/log"
	"os"
)

type Paths struct {
	cfg            *config.Config
	scratchRootDir string
}

/* Constructor for Paths object. */
func NewPaths(cfg *config.Config) (*Paths, error) {
	scratchDir, err := os.MkdirTemp(cfg.Tmpdir(), "thelma-scratch")
	if err != nil {
		return nil, err
	}
	paths := &Paths{
		cfg:            cfg,
		scratchRootDir: scratchDir,
	}
	return paths, nil
}

func (p *Paths) CreateScratchDir(nickname string) (string, error) {
	dir, err := os.MkdirTemp(p.scratchRootDir, nickname)
	if err != nil {
		return "", err
	}
	log.Debug().Msgf("Created scratch directory %s", dir)
	return dir, nil
}

func (p *Paths) Cleanup() error {
	log.Debug().Msgf("Deleting scratch root directory %s", p.scratchRootDir)
	return os.RemoveAll(p.scratchRootDir)
}