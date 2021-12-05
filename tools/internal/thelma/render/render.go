package render

import (
	"context"
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/helmfile"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/resolver"
	"github.com/rs/zerolog/log"
	"strings"
	"sync"
	"time"
)

// multiRenderTimeout how long to wait before timing out a multi render
const multiRenderTimeout = 5 * time.Minute

// Options encapsulates CLI options for a render
type Options struct {
	Env             *string       // Env If supplied, render for a single environment instead of all targets
	Cluster         *string       // Cluster If supplied, render for a single cluster instead of all targets
	Release         *string       // Release If supplied, render only the specified release
	Stdout          bool          // Stdout if true, render to stdout instead of output directory
	OutputDir       string        // Output directory where manifests should be rendered
	ChartSourceDir  string        // Path on filesystem where chart sources live
	ResolverMode    resolver.Mode // Resolver mode
	ParallelWorkers int           // ParallelWorkers Number of parallel workers
}

// multiRender renders manifests for multiple environments and clusters
type multiRender struct {
	options    *Options // Options global render options
	gitops     gitops.Gitops
	configRepo *helmfile.ConfigRepo // configRepo refernce to use for executing `helmfile template`
}

// renderError represents an error encountered while rendering for a particular target
type renderError struct {
	description string // description for the render job that generated in this error
	err         error  // error
}

// renderJob represents a single helmfile invocation with parameters
type renderJob struct {
	description string
	callback    func() error
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

	_gitops, err := gitops.Load(app.Config.Home(), app.ShellRunner)
	if err != nil {
		return nil, err
	}
	r.gitops = _gitops

	chartCacheDir, err := app.Paths.CreateScratchDir("chart-cache")
	if err != nil {
		return nil, err
	}

	scratchDir, err := app.Paths.CreateScratchDir("helmfile-scratch")
	if err != nil {
		return nil, err
	}

	helmfileLogLevel := "info"
	if app.Config.LogLevel() == "trace" {
		helmfileLogLevel = "debug"
	}

	r.configRepo = helmfile.NewConfigRepo(helmfile.Options{
		ThelmaHome:       app.Config.Home(),
		ChartCacheDir:    chartCacheDir,
		ChartSourceDir:   options.ChartSourceDir,
		ResolverMode:     options.ResolverMode,
		HelmfileLogLevel: helmfileLogLevel,
		Stdout:           options.Stdout,
		OutputDir:        options.OutputDir,
		ScratchDir:       scratchDir,
		ShellRunner:      app.ShellRunner,
	})

	return r, nil
}

// renderAll renders manifests based on supplied arguments
func (r *multiRender) renderAll(helmfileArgs *helmfile.Args) error {
	jobs, err := r.getJobs(helmfileArgs)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return fmt.Errorf("no matching releases found")
	}

	numWorkers := 1
	if r.options.ParallelWorkers >= 1 {
		numWorkers = r.options.ParallelWorkers
	}
	if len(jobs) < numWorkers {
		// don't make more workers than we have items to process
		numWorkers = len(jobs)
	}

	log.Info().Msgf("Rendering %d release(s) with %d worker(s)", len(jobs), numWorkers)

	queueCh := make(chan renderJob, len(jobs))
	for _, job := range jobs {
		queueCh <- job
	}
	close(queueCh)

	errCh := make(chan renderError, len(jobs))

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		id := i
		wg.Add(1)

		logger := log.With().Str("wid", fmt.Sprintf("worker-%d", id)).Logger()

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					logger.Debug().Msg("short circuit triggered, returning")
					return

				case job, ok := <-queueCh:
					if !ok {
						logger.Debug().Msg("no jobs left, returning")
						return
					}

					logger.Debug().Msgf("rendering %s", job.description)
					if err := job.callback(); err != nil {
						logger.Error().Msgf("error rendering %s:\n%v", job.description, err)
						errCh <- renderError{
							description: job.description,
							err:         err,
						}
						cancel()
						return
					}
				}
			}
		}()
	}

	// Wait for workers to finish in a separate goroutine so that we can implement
	// a timeout
	waitCh := make(chan struct{})
	go func() {
		logger := log.With().Str("wid", "wait").Logger()
		logger.Debug().Msg("Waiting for render workers to finish")
		wg.Wait()
		logger.Debug().Msgf("Render workers finished")
		close(errCh)
		close(waitCh)
	}()

	// Block until the wait group is done or we time out.
	logger := log.With().Str("wid", "main").Logger()

	select {
	case <-waitCh:
		logger.Debug().Msgf("multi-render finished")
	case <-time.After(multiRenderTimeout):
		err := fmt.Errorf("[main] multi-render timed out after %s", multiRenderTimeout)
		logger.Error().Err(err)
		return err
	}

	return readErrorsFromChannel(errCh)
}

// getJobs returns the set of render jobs that should be run
func (r *multiRender) getJobs(helmfileArgs *helmfile.Args) ([]renderJob, error) {
	filter := gitops.AnyRelease()
	targets := r.gitops.Targets()
	releaseScoped := false

	if r.options.Release != nil {
		log.Debug().Msgf("Filtering for releases with name: %s", *r.options.Release)
		filter = filter.And(gitops.HasName(*r.options.Release))
		releaseScoped = true
	}

	if r.options.Env != nil {
		envName := *r.options.Env
		log.Debug().Msgf("Filtering for releases in env: %s", envName)
		filter = filter.And(gitops.HasTarget(envName))
		targets = []gitops.Target{r.gitops.GetTarget(envName)}
	}

	if r.options.Cluster != nil {
		clusterName := *r.options.Cluster
		log.Debug().Msgf("Filtering for releases in cluster: %s", clusterName)
		filter = filter.And(gitops.HasTarget(clusterName))
		targets = []gitops.Target{r.gitops.GetTarget(clusterName)}
	}

	releases := r.gitops.FilterReleases(filter)
	log.Debug().Msgf("%d releases matched filter: %v", len(releases), releases)

	var jobs []renderJob

	for _, release := range releases {
		_r := release
		jobs = append(jobs, renderJob{
			description: fmt.Sprintf("release %s in %s %s", _r.Name(), _r.Target().Type(), _r.Target().Name()),
			callback: func() error {
				return r.configRepo.RenderForRelease(_r, helmfileArgs)
			},
		})
	}

	if !releaseScoped {
		for _, _target := range targets {
			t := _target
			jobs = append(jobs, renderJob{
				description: fmt.Sprintf("global resources for %s %s", t.Type(), t.Name()),
				callback: func() error {
					return r.configRepo.RenderForTarget(t, helmfileArgs)
				},
			})
		}
	}

	return jobs, nil
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
		description := renderErr.description
		err := renderErr.err
		sb.WriteString(fmt.Sprintf("%s: %v\n", description, err))
	}

	if count > 0 {
		return fmt.Errorf("%d render errors:\n%s", count, sb.String())
	}

	return nil
}
