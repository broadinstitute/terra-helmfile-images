package app

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app/config"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app/paths"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
)

// Options for a ThelmaApp
type Options struct {
	Runner shell.Runner
}

// ThelmaApp Cross-cutting dependencies for Thelma commands
type ThelmaApp struct {
	Config      *config.Config
	ShellRunner shell.Runner
	Paths       *paths.Paths
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
	_paths, err := paths.New(cfg)
	if err != nil {
		return nil, err
	}
	app.Paths = _paths

	// Initialize ShellRunner
	if options.Runner != nil {
		app.ShellRunner = options.Runner
	} else {
		app.ShellRunner = shell.NewDefaultRunner()
	}

	return app, nil
}
