package render

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops/target"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/helmfile"
	"github.com/rs/zerolog/log"
	"strings"
	"sync"
	"time"
)

// multiRenderTimeout how long to wait before timing out a multi render
const multiRenderTimeout = 5 * time.Minute

// Options encapsulates CLI options for a render
type Options struct {
	Env             *string  // Env If supplied, render for a single environment instead of all targets
	Cluster         *string  // Cluster If supplied, render for a single cluster instead of all targets
	Stdout     		 bool    // Stdout if true, render to stdout instead of output directory
	OutputDir        string  // Output directory where manifests should be rendered
	ParallelWorkers int      // ParallelWorkers Number of parallel workers
}

// multiRender renders manifests for multiple environments and clusters
type multiRender struct {
	options           *Options                        // Options global render options
	configuredTargets map[string]target.ReleaseTarget // Collection of release targets (environments and clusters) defined in the config repo, keyed by name
	configRepo        *helmfile.ConfigRepo            // configRepo refernce to use for executing `helmfile template`
}

// renderError represents an error encountered while rendering for a particular target
type renderError struct {
	target target.ReleaseTarget // release target that resulted in this error
	err    error                // error
}

// DoRender constructs a multiRender and invokes all functions in correct order to perform a complete
// render.
func DoRender(app *app.ThelmaApp, globalOptions *Options, helmfileArgs *helmfile.Args) error {
	r, err := newRender(app, globalOptions)
	if err != nil {
		return err
	}
	if err = r.configRepo.CleanOutputDirectoryIfEnabled(); err != nil {
		return err
	}
	if err = r.configRepo.HelmUpdate(); err != nil {
		return err
	}
	if err = r.renderAll(helmfileArgs); err != nil {
		return err
	}
	return nil
}

// newRender is a constructor for Render objects
func newRender(app *app.ThelmaApp, options *Options) (*multiRender, error) {
	r := new(multiRender)
	r.options = options

	targets, err := target.LoadReleaseTargets(app.Config.Home())
	if err != nil {
		return nil, err
	}
	r.configuredTargets = targets

	chartCacheDir, err := app.Paths.CreateScratchDir("chart-cache")
	if err != nil {
		return nil, err
	}

	r.configRepo = helmfile.NewConfigRepo(helmfile.Options{
		Path:             app.Config.Home(),
		ChartCacheDir:    chartCacheDir,
		HelmfileLogLevel: app.Config.LogLevel(),
		Stdout: options.Stdout,
		OutputDir: options.OutputDir,
		ShellRunner:      app.ShellRunner,
	})

	return r, nil
}

// renderAll renders manifests based on supplied arguments
func (r *multiRender) renderAll(helmfileArgs *helmfile.Args) error {
	releaseTargets, err := r.getTargets()
	if err != nil {
		return err
	}

	log.Info().Msgf("Rendering manifests for %d target(s)...", len(releaseTargets))

	numWorkers := 1
	if r.options.ParallelWorkers >= 1 {
		numWorkers = r.options.ParallelWorkers
	}
	if len(releaseTargets) < numWorkers {
		// don't make more workers than we have items to process
		numWorkers = len(releaseTargets)
	}

	queueCh := make(chan target.ReleaseTarget, len(releaseTargets))
	for _, releaseTarget := range releaseTargets {
		queueCh <- releaseTarget
	}
	close(queueCh)

	errCh := make(chan renderError, len(releaseTargets))

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		id := i
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				releaseTarget, ok := <-queueCh
				if !ok {
					log.Debug().Msgf("[worker-%d] queue channel closed, returning", id)
					return
				}

				log.Debug().Msgf("[worker-%d] rendering target %s", id, releaseTarget.Name())
				if err := r.renderSingleTarget(helmfileArgs, releaseTarget); err != nil {
					log.Error().Msgf("[worker-%d] error rendering manifests for target %s:\n%v", id, releaseTarget.Name(), err)
					errCh <- renderError{
						target: releaseTarget,
						err:    err,
					}
				}
			}
		}()
	}

	// Wait for workers to finish in a separate goroutine so that we can implement
	// a timeout
	waitCh := make(chan struct{})
	go func() {
		log.Debug().Msgf("[wait] Waiting for render workers to finish")
		wg.Wait()
		log.Debug().Msgf("[wait] Render workers finished")
		close(errCh)
		close(waitCh)
	}()

	// Block until the wait group is done or we time out.
	select {
	case <-waitCh:
		log.Debug().Msgf("[main] multi-render finished")
	case <-time.After(multiRenderTimeout):
		err := fmt.Errorf("[main] multi-render timed out after %s", multiRenderTimeout)
		log.Error().Err(err)
		return err
	}

	return readErrorsFromChannel(errCh)
}

// RenderAll renders manifests based on supplied arguments
func (r *multiRender) renderSingleTarget(helmfileArgs *helmfile.Args, releaseTarget target.ReleaseTarget) error {
	return r.configRepo.Render(releaseTarget, helmfileArgs)
}

// getTargets returns the set of release targets to render manifests for
func (r *multiRender) getTargets() ([]target.ReleaseTarget, error) {
	if r.options.Env != nil {
		// User wants to render for a specific environment
		env, ok := r.configuredTargets[*r.options.Env]
		if !ok {
			return nil, fmt.Errorf("unknown environment: %s", *r.options.Env)
		}
		return []target.ReleaseTarget{env}, nil
	}

	if r.options.Cluster != nil {
		// User wants to render for a specific cluster
		cluster, ok := r.configuredTargets[*r.options.Cluster]
		if !ok {
			return nil, fmt.Errorf("unknown cluster: %s", *r.options.Cluster)
		}
		return []target.ReleaseTarget{cluster}, nil
	}

	// No target specified; render for _all_ targets
	var targets []target.ReleaseTarget
	for _, releaseTarget := range r.configuredTargets {
		targets = append(targets, releaseTarget)
	}

	// Sort targets so they are rendered in a predictable order
	target.SortReleaseTargets(targets)

	return targets, nil
}

// readErrorsFromChannel aggregates all errors into a single mega-error
func readErrorsFromChannel(errCh <-chan renderError) error {
	var count int
	var sb strings.Builder

	for {
		renderErr, ok := <-errCh
		if !ok {
			// channel closed, no more results to read
			break
		}
		count++
		releaseTarget := renderErr.target
		err := renderErr.err
		sb.WriteString(fmt.Sprintf("%s %s: %v\n", releaseTarget.Type(), releaseTarget.Name(), err))
	}

	if count > 0 {
		return fmt.Errorf("Failed to render manifests for %d targets:\n%s", count, sb.String())
	}

	return nil
}
