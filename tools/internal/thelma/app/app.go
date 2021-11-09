package app

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/config"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/paths"
)

type Options struct {
	Runner shell.Runner
}

// Cross-cutting dependencies for Thelma
type ThelmaApp struct {
	Config *config.Config
	ShellRunner shell.Runner
	Paths *paths.Paths
}

// New construct a new App, given a Config
func New(cfg *config.Config) (*ThelmaApp, error) {
	return NewWithOptions(cfg, Options{})
}

// NewWithOptions construct a new App, given a Config & options
func NewWithOptions(cfg *config.Config, options Options) (*ThelmaApp, error) {
	app := &ThelmaApp{}
	app.Config = cfg

	// Initialize paths
	_paths, err := paths.NewPaths(cfg)
	if err != nil {
		return nil, err
	}
	app.Paths = _paths

	// Initialize ShellRunner
	if options.Runner != nil {
		app.ShellRunner = options.Runner
	} else {
		app.ShellRunner = shell.NewRealRunner()
	}

	return app, nil
}