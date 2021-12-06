package helmfile

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/helmfile/argocd"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/resolver"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

const cmdLogLevel = zerolog.DebugLevel

// Args arguments for a helmfile render
type Args struct {
	ChartVersion *string  // ChartVersion optionally override chart version
	AppVersion   *string  // AppVersion optionally override application version (container image)
	ValuesFiles  []string // ValuesFiles optional list of files for overriding chart values
	ArgocdMode   bool     // ArgocdMode if true, render ArgoCD manifests instead of application manifests
}

// Options constructor arguments for a ConfigRepo
type Options struct {
	ThelmaHome       string        // ThelmaHome to terra-helmfile repo clone
	ChartSourceDir   string        // ChartSourceDir path on filesystem containing chart sources
	ChartCacheDir    string        // ChartCacheDir path to shared chart cache directory that can be re-used across renders
	ResolverMode     resolver.Mode // Resolver mode
	HelmfileLogLevel string        // HelmfileLogLevel is the --log-level argument to pass to `helmfile` command
	Stdout           bool          // Stdout if true, render to stdout instead of output directory
	OutputDir        string        // Output directory where manifests should be rendered
	ScratchDir       string        // Scratch directory where temporary files should be written
	ShellRunner      shell.Runner  // ShellRunner shell Runner to use for executing helmfile commands
}

// ConfigRepo can be used to `helmfile` render commands on a clone of the `terra-helmfile` repo
type ConfigRepo struct {
	thelmaHome       string
	chartResolver    resolver.Resolver
	helmfileLogLevel string
	stdout           bool
	outputDir        string
	scratchDir       string
	shellRunner      shell.Runner
}

// NewConfigRepo constructs a new ConfigRepo object
func NewConfigRepo(options Options) *ConfigRepo {
	chartResolver := resolver.NewResolver(options.ShellRunner, resolver.Options{
		Mode:       options.ResolverMode,
		CacheDir:   options.ChartCacheDir,
		SourceDir:  options.ChartSourceDir,
		ScratchDir: path.Join(options.ScratchDir, "resolver"),
	})

	return &ConfigRepo{
		thelmaHome:       options.ThelmaHome,
		chartResolver:    chartResolver,
		helmfileLogLevel: options.HelmfileLogLevel,
		stdout:           options.Stdout,
		outputDir:        options.OutputDir,
		scratchDir:       path.Join(options.ScratchDir, "helmfile"),
		shellRunner:      options.ShellRunner,
	}
}

// CleanOutputDirectory cleans the output directory before rendering
func (r *ConfigRepo) CleanOutputDirectoryIfEnabled() error {
	if r.stdout {
		// No need to clean output directory if we're rendering to stdout
		return nil
	}

	if _, err := os.Stat(r.outputDir); os.IsNotExist(err) {
		// Output dir does not exist, nothing to clean up
		return nil
	}

	// Delete any files that exist inside the output directory.
	log.Debug().Msgf("Cleaning output directory: %s", r.outputDir)

	// This code would be simpler if we could just call os.RemoveAll() on the
	// output directory itself, but in some cases the output directory is a volume
	// mount in a Docker container, and trying to remove it throws an error.
	// So we remove all its contents instead.
	dir, err := ioutil.ReadDir(r.outputDir)
	if err != nil {
		return err
	}

	for _, file := range dir {
		filePath := path.Join([]string{r.outputDir, file.Name()}...)
		log.Debug().Msgf("Deleting %s", filePath)

		err = os.RemoveAll(filePath)
		if err != nil {
			return err
		}
	}

	return nil
}

// HelmUpdate updates Helm repo indexes.
func (r *ConfigRepo) HelmUpdate() error {
	log.Info().Msg("Updating Helm repo indexes")

	cmd := shell.Command{
		Prog: ProgName,
		Args: []string{
			fmt.Sprintf("--log-level=%s", r.helmfileLogLevel),
			"--allow-no-matching-release",
			"repos",
		},
		Dir: r.thelmaHome,
	}

	return r.runCmd(cmd)
}

func (r *ConfigRepo) RenderForTarget(target gitops.Target, args *Args) error {
	if args.ArgocdMode {
		if len(target.Releases()) == 0 {
			log.Debug().Msgf("%s %s has no releases, won't render ArgoCD project", target.Type(), target.Name())
		} else {
			return r.renderArgocdProjectManifests(target)
		}
	}
	return nil
}

func (r *ConfigRepo) RenderForRelease(release gitops.Release, args *Args) error {
	if args.ArgocdMode {
		return r.renderArgocdApplicationManifests(release)
	} else {
		return r.renderApplicationManifests(release, args)
	}
}

// Render Argo project manifests for the given target
func (r *ConfigRepo) renderArgocdProjectManifests(target gitops.Target) error {
	outputDir := path.Join(r.outputDir, target.Name(), "terra-argocd-project")

	values := argocd.GetDestinationValues(target)
	valuesFile, err := r.writeTemporaryValuesFile(values, target)
	if err != nil {
		return err
	}

	cmd := newCmd()
	cmd.setOutputDir(outputDir)
	cmd.setStdout(r.stdout)
	cmd.setDir(path.Join(r.thelmaHome, "argocd", "project"))
	cmd.setLogLevel(r.helmfileLogLevel)
	cmd.setSkipDeps(true) // argocd project chart is local & has no dependencies
	cmd.addValuesFiles(valuesFile)

	cmd.setTargetEnvVars(target)
	cmd.setArgocdProjectEnvVar(target)

	log.Info().Msgf("Rendering ArgoCD manifests for %s %s", target.Name(), target.Type())

	return r.runHelmfile(cmd)
}

// Render Argo manifests for the given release
func (r *ConfigRepo) renderArgocdApplicationManifests(release gitops.Release) error {
	dir := fmt.Sprintf("terra-argocd-app-%s", release.Name())
	outputDir := path.Join(r.outputDir, release.Target().Name(), dir)

	cmd := newCmd()
	cmd.setOutputDir(outputDir)
	cmd.setStdout(r.stdout)
	cmd.setDir(path.Join(r.thelmaHome, "argocd", "application"))
	cmd.setLogLevel(r.helmfileLogLevel)
	cmd.setSkipDeps(true) // argocd application chart is local & has no dependencies

	cmd.setReleaseEnvVars(release)
	cmd.setTargetEnvVars(release.Target())
	cmd.setArgocdProjectEnvVar(release.Target())
	cmd.setNamespaceEnvVar(release)
	cmd.setClusterEnvVars(release)

	log.Info().Msgf("Rendering ArgoCD manifests for %s in %s", release.Name(), release.Target().Name())

	return r.runHelmfile(cmd)
}

// Render application manifests for the given release
func (r *ConfigRepo) renderApplicationManifests(release gitops.Release, args *Args) error {
	chartVersion := release.ChartVersion()
	if args.ChartVersion != nil {
		log.Debug().Msgf("Overriding default chart version for %s with %s", chartVersion, *args.ChartVersion)
		chartVersion = *args.ChartVersion
	}

	resolvedChart, err := r.chartResolver.Resolve(resolver.ChartRelease{
		Name:    release.ChartName(),
		Repo:    release.Repo(),
		Version: chartVersion,
	})
	if err != nil {
		return fmt.Errorf("error resolving chart for release %s in %s %s: %v", release.Name(), release.Target().Type(), release.Target().Name(), err)
	}

	outputDir := path.Join(r.outputDir, release.Target().Name(), release.Name())

	cmd := newCmd()
	cmd.setOutputDir(outputDir)
	cmd.setStdout(r.stdout)
	cmd.setDir(r.thelmaHome)
	cmd.setLogLevel(r.helmfileLogLevel)

	// resolver runs `helm dependency update` on local charts, so we always set --skip-deps to save time
	cmd.setSkipDeps(true)

	cmd.setReleaseEnvVars(release)
	cmd.setTargetEnvVars(release.Target())
	cmd.setNamespaceEnvVar(release)

	cmd.addValuesFiles(args.ValuesFiles...)
	cmd.setChartPathEnvVar(resolvedChart.Path())

	logEvent := log.Info().
		Str("chartVersion", resolvedChart.Version()).
		Str("chartSource", resolvedChart.SourceDescription())

	if release.Type() == gitops.AppReleaseType {
		appVersion := release.(gitops.AppRelease).AppVersion()
		if args.AppVersion != nil {
			log.Debug().Msgf("Overriding default app version %s with %s", appVersion, *args.AppVersion)
			appVersion = *args.AppVersion
		}
		logEvent = logEvent.Str("appVersion", appVersion)
		cmd.setAppVersionEnvVar(appVersion)

	} else if args.AppVersion != nil {
		log.Warn().Msgf("Ignoring --app-version %s; --app-version is not supported for cluster releases", *args.AppVersion)
	}

	logEvent.Msgf("Rendering %s in %s", release.Name(), release.Target().Name())

	return r.runHelmfile(cmd)
}

func (r *ConfigRepo) runHelmfile(cmd *Cmd) error {
	err := r.runCmd(cmd.toShellCommand())
	if err != nil {
		return err
	}

	if !r.stdout {
		return normalizeOutputDir(cmd.outputDir)
	}

	return nil
}

func (r *ConfigRepo) runCmd(cmd shell.Command) error {
	level := cmdLogLevel

	var stdoutWriter io.Writer
	if r.stdout {
		stdoutWriter = os.Stdout
	}
	return r.shellRunner.RunWith(cmd, shell.RunOptions{
		LogLevel: &level,
		Stdout: stdoutWriter,
	})
}

// Normalize output directories so they match what was produced by earlier iterations of the render tool.
//
// Old behavior
// ------------
// For regular releases:
//  output/dev/helmfile-b47efc70-leonardo/leonardo
//  ->
//  output/dev/leonardo/leonardo
//
// For ArgoCD:
//  output/dev/helmfile-b47efc70-terra-argocd-app-leonardo/terra-argocd-app
//  ->
//  output/dev/terra-argocd-app-leonardo/terra-argocd-app
//
//  output/dev/helmfile-b47efc70-terra-argocd-project/terra-argocd-project
//  ->
//  output/dev/terra-argocd-project/terra-argocd-project
//
// New behavior
// ------------
// For regular releases:
//  output/dev/leonardo/helmfile-b47efc70-leonardo/leonardo
//  ->
//  output/dev/leonardo/leonardo
//
// For ArgoCD:
//  output/dev/terra-argocd-app-leonardo/helmfile-b47efc70-terra-argocd-app-leonardo/terra-argocd-app
//  ->
//  output/dev/terra-argocd-app-leonardo/terra-argocd-app
//
//  output/dev/terra-argocd-project/helmfile-b47efc70-terra-argocd-project/terra-argocd-project
//  ->
//  output/dev/terra-argocd-project/terra-argocd-project
//
// normalizeOutputDir removes "helmfile-.*" directories from helmfile output paths.
// this makes it possible to easily run diff -r on render outputs from different branches
func normalizeOutputDir(outputDir string) error {
	glob := path.Join(outputDir, "helmfile-*", "*")
	matches, err := filepath.Glob(glob)
	if err != nil {
		return fmt.Errorf("error globbing rendered templates %s: %v", glob, err)
	}

	if len(matches) != 1 {
		return fmt.Errorf("expected exactly one match for %s, got %d: %v", glob, len(matches), matches)
	}

	match := matches[0]
	dest := path.Join(path.Dir(path.Dir(match)), path.Base(match))
	log.Debug().Msgf("Renaming %s to %s", match, dest)

	if err := os.Rename(match, dest); err != nil {
		return err
	}
	if err := os.Remove(path.Dir(match)); err != nil {
		return err
	}

	return nil
}

// Convert structured data to YAML and write to the given file
func (r *ConfigRepo) writeTemporaryValuesFile(values interface{}, target gitops.Target) (string, error) {
	filename := path.Join(r.scratchDir, target.Name(), "values.yaml")
	if err := os.MkdirAll(path.Dir(filename), 0775); err != nil {
		return "", fmt.Errorf("error writing temporary values file %s for %s %s: %v", filename, target.Type(), target.Name(), err)

	}
	output, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("error marshaling values to YAML for %s %s: %v (content: %v)", target.Type(), target.Name(), err, values)
	}

	if err := os.WriteFile(filename, output, 0666); err != nil {
		return "", fmt.Errorf("error writing temporary values file %s for %s %s: %v", filename, target.Type(), target.Name(), err)
	}

	return filename, nil
}
