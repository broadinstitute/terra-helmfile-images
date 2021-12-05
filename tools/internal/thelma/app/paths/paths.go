package paths

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app/config"
	"github.com/rs/zerolog/log"
	"os"
	"path"
)

const miscConfDir = "etc"
const defaultChartSrcDir = "charts"

// Paths is a utility for interacting with terra-helmfile paths
type Paths struct {
	cfg            *config.Config
	scratchRootDir string
}

// New constructor for Paths object.
func New(cfg *config.Config) (*Paths, error) {
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

// DefaultChartSrcDir default directory in terra-helmfile where chart sources live
func (p *Paths) DefaultChartSrcDir() string {
	return path.Join(p.cfg.Home(), defaultChartSrcDir)
}

// MiscConfDir directory in terra-helmfile containing miscellaneous config files
func (p *Paths) MiscConfDir() string {
	return path.Join(p.cfg.Home(), miscConfDir)
}

// CreateScratchDir creates a new temporary directory
func (p *Paths) CreateScratchDir(nickname string) (string, error) {
	dir, err := os.MkdirTemp(p.scratchRootDir, nickname)
	if err != nil {
		return "", err
	}
	log.Debug().Msgf("Created scratch directory %s", dir)
	return dir, nil
}

// Cleanup will clean up all temporary/scratch directories
func (p *Paths) Cleanup() error {
	log.Debug().Msgf("Deleting scratch root directory %s", p.scratchRootDir)
	return os.RemoveAll(p.scratchRootDir)
}
