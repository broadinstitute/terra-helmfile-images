package helmfile

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ProgName is the name of the `helmfile` binary
const ProgName = "helmfile"

// Environment variables -- prefixed with THF for "terra-helmfile"

// TargetTypeEnvVar is the name of the environment variable used to pass target type to helmfile
const TargetTypeEnvVar = "THF_TARGET_TYPE"

// TargetBaseEnvVar is the name of the environment variable used to pass target base to helmfile
const TargetBaseEnvVar = "THF_TARGET_BASE"

// TargetNameEnvVar is the name of the environment variable used to pass target name to helmfile
const TargetNameEnvVar = "THF_TARGET_NAME"

// ChartCacheDirEnvVar is the name of the environment variable used to pass unpack dir to helmfile
const ChartCacheDirEnvVar = "THF_CHART_CACHE_DIR"

// ChartSrcDirEnvVar is the name of the environment variable used to pass --chart-dir overrides to helmfile
const ChartSrcDirEnvVar = "THF_CHART_SRC_DIR"

// localChartVersion is what we set chart version to when rendering from a chart on the local filesystem
// instead of the official, published chart version
const localChartVersion = "local"

// Args arguments for a helmfile render
type Args struct {
	ChartVersion *string  // ChartVersion optionally override chart version
	AppVersion   *string  // AppVersion optionally override application version (container image)
	ChartDir     *string  // ChartDir optionally render chart from the given directory instead of the given release
	ValuesFiles  []string // ValuesFiles optional list of files for overriding chart values
	ArgocdMode   bool     // ArgocdMode if true, render ArgoCD manifests instead of application manifests
}

// Options constructor arguments for a ConfigRepo
type Options struct {
	Path             string       // Path to terra-helmfile repo clone
	ChartCacheDir    string       // ChartCacheDir path to shared chart cache directory that can be re-used across renders
	HelmfileLogLevel string       // HelmfileLogLevel is the --log-level argument to pass to `helmfile` command
	Stdout     		 bool         // Stdout if true, render to stdout instead of output directory
	OutputDir        string       // Output directory where manifests should be rendered
	ShellRunner      shell.Runner // ShellRunner shell Runner to use for executing helmfile commands
}

// ConfigRepo can be used to `helmfile` render commands on a clone of the `terra-helmfile` repo
type ConfigRepo struct {
	path             string
	chartCacheDir    string
	helmfileLogLevel string
	stdout bool
	outputDir string
	shellRunner      shell.Runner
}

// helmfileParams encapsulates low-level parameters for a `helmfile` command
type helmfileParams struct {
	envVars     []string
	stateValues map[string]string
	selectors   map[string]string
}

// NewConfigRepo constructs a new ConfigRepo object
func NewConfigRepo(options Options) *ConfigRepo {
	return &ConfigRepo{
		path:             options.Path,
		chartCacheDir:    options.ChartCacheDir,
		helmfileLogLevel: options.HelmfileLogLevel,
		stdout: options.Stdout,
		outputDir: options.OutputDir,
		shellRunner:      options.ShellRunner,
	}
}

// newHelmfileParams returns a new helmfileParams object with all fields initialized
func newHelmfileParams() *helmfileParams {
	return &helmfileParams{
		stateValues: make(map[string]string),
		selectors:   make(map[string]string),
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
	// mount, and trying to remove it throws an error.
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

// TODO: move to chart cache
// HelmUpdate updates Helm repos
func (r *ConfigRepo) HelmUpdate() error {
	log.Debug().Msg("Updating Helm repos...")
	return r.runHelmfile([]string{}, "--allow-no-matching-release", "repos")
}

// Render renders manifests for the given target
func (r *ConfigRepo) RenderGlobalTargetResources(target gitops.Target, helmfileArgs *Args) error {
	if !helmfileArgs.ArgocdMode {
		// nothing to do -- ArgoCD projects are the only global resources
		return nil
	}
	outputDir := path.Join(r.outputDir, target.Name(), "terra-argocd-project")

	params := newHelmfileParams()
	params.setTargetEnvVars(target)
	params.setChartCacheDirEnvVar(r.chartCacheDir)
	params.applyTargetScopeSelectors(helmfileArgs)

	log.Info().Msgf("Rendering manifests for argo project in %s", target.Name())

	return r.renderWithParams(params, helmfileArgs, outputDir)
}

// Render renders manifests for the given target
func (r *ConfigRepo) RenderRelease(release gitops.Release, helmfileArgs *Args) error {
	dir := release.Name()
	if helmfileArgs.ArgocdMode {
		dir = fmt.Sprintf("terra-argocd-app-%s", release.Name())
	}
	outputDir := path.Join(r.outputDir, release.Target().Name(), dir)

	params := newHelmfileParams()
	params.setTargetEnvVars(release.Target())
	params.setChartCacheDirEnvVar(r.chartCacheDir)
	params.applyReleaseScopeSelectors(release.Name(), helmfileArgs)
	params.applyVersionSettings(release.Name(), helmfileArgs)

	log.Info().Msgf("Rendering manifests for release %s in %s", release.Name(), release.Target().Name())

	return r.renderWithParams(params, helmfileArgs, outputDir)
}

func (r *ConfigRepo) renderWithParams(params *helmfileParams, args *Args, outputDir string) error {
	// Convert helmfile parameters into cli arguments
	var cliArgs []string

	if len(params.selectors) != 0 {
		selectorString := joinKeyValuePairs(params.selectors)
		cliArgs = append(cliArgs, fmt.Sprintf("--selector=%s", selectorString))
	}

	if len(params.stateValues) != 0 {
		stateValuesString := joinKeyValuePairs(params.stateValues)
		cliArgs = append(cliArgs, fmt.Sprintf("--state-values-set=%s", stateValuesString))
	}

	// Append Helmfile command we're running (template)
	cliArgs = append(cliArgs, "template")

	// Append arguments specific to template subcommand
	if args.ChartDir == nil {
		// Skip dependencies unless we're rendering a local chart, to save time
		cliArgs = append(cliArgs, "--skip-deps")
	}
	if len(args.ValuesFiles) > 0 {
		cliArgs = append(cliArgs, fmt.Sprintf("--values=%s", strings.Join(args.ValuesFiles, ",")))
	}

	if !r.stdout {
		outputDirFlag := fmt.Sprintf("--output-dir=%s", outputDir)
		cliArgs = append(cliArgs, outputDirFlag)
	}

	err := r.runHelmfile(params.envVars, cliArgs...)
	if err != nil {
		return err
	}

	if !r.stdout {
		if err := normalizeOutputDir(outputDir); err != nil {
			return err
		}
	}

	return nil
}

func (r *ConfigRepo) runHelmfile(envVars []string, args ...string) error {
	finalArgs := []string{
		fmt.Sprintf("--log-level=%s", r.helmfileLogLevel),
	}
	finalArgs = append(finalArgs, args...)

	cmd := shell.Command{}

	if len(envVars) > 0 {
		cmd.Env = envVars
	}

	cmd.Prog = ProgName
	cmd.Args = finalArgs
	cmd.Dir = r.path

	return r.shellRunner.Run(cmd)
}

// setTargetEnvVars sets environment variables for the given target of environment variables
// Eg.
// {"TARGET_TYPE=environment", "TARGET_BASE=live", "TARGET_NAME=dev"}
func (params *helmfileParams) setTargetEnvVars(t gitops.Target) {
	params.addEnvVar(TargetTypeEnvVar, t.Type().String())
	params.addEnvVar(TargetBaseEnvVar, t.Base())
	params.addEnvVar(TargetNameEnvVar, t.Name())
}

// setChartCacheDirEnvVar sets the chart cache dir env var
func (params *helmfileParams) setChartCacheDirEnvVar(chartCacheDir string) {
	params.addEnvVar(ChartCacheDirEnvVar, chartCacheDir)
}

// applyVersionSettings updates params with necessary state values & environment variables, based on
// the configured chart version, chart dir, & app version
func (params *helmfileParams) applyVersionSettings(releaseName string, helmfileArgs *Args) {
	if helmfileArgs.ChartDir != nil {
		params.addEnvVar(ChartSrcDirEnvVar, *helmfileArgs.ChartDir)

		key := fmt.Sprintf("releases.%s.chartVersion", releaseName)
		params.stateValues[key] = localChartVersion
	} else if helmfileArgs.ChartVersion != nil {
		key := fmt.Sprintf("releases.%s.chartVersion", releaseName)
		params.stateValues[key] = *helmfileArgs.ChartVersion
	}

	if helmfileArgs.AppVersion != nil  {
		key := fmt.Sprintf("releases.%s.appVersion", releaseName)
		params.stateValues[key] = *helmfileArgs.AppVersion
	}
}

// populates selectors with correct values based on the ArgocdMode and Release arguments.
func (params *helmfileParams) applyReleaseScopeSelectors(releaseName string, helmfileArgs *Args) {
	params.selectors["mode"] = "release"
	if helmfileArgs.ArgocdMode {
		// Render ArgoCD manifests instead of application manifests
		params.selectors["mode"] = "argocd"
	}
	params.selectors["release"] = releaseName
	params.selectors["scope"] = "release"
}

// populates selectors with correct values based on the ArgocdMode and Release arguments.
func (params *helmfileParams) applyTargetScopeSelectors(helmfileArgs *Args) {
	params.selectors["mode"] = "release"
	if helmfileArgs.ArgocdMode {
		// Render ArgoCD manifests instead of application manifests
		params.selectors["mode"] = "argocd"
	}
	params.selectors["scope"] = "target"
}

// addEnvVar adds an env var key/value pair to the given params instance
func (params *helmfileParams) addEnvVar(name string, value string) {
	params.envVars = append(params.envVars, fmt.Sprintf("%s=%s", name, value))
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
//
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
//
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
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	glob := path.Join(outputDir, "helmfile-*", "*")
	matches, err := filepath.Glob(glob)
	if err != nil {
		return fmt.Errorf("error globbing rendered templates %s: %v", glob, err)
	}

	for _, match := range matches {
		dest := path.Join(outputDir, path.Base(match))
		log.Debug().Msgf("Renaming %s to %s", match, dest)
		if err := os.Rename(match, dest); err != nil {
			return err
		}
		if err := os.Remove(path.Dir(match)); err != nil {
			return err
		}
	}

	return nil
}

// joinKeyValuePairs joins map[string]string to string containing comma-separated key-value pairs.
// Eg. { "a": "b", "c": "d" } -> "a=b,c=d"
func joinKeyValuePairs(pairs map[string]string) string {
	var tokens []string
	for k, v := range pairs {
		tokens = append(tokens, fmt.Sprintf("%s=%s", k, v))
	}

	// Sort tokens so they are always supplied in predictable order
	sort.Strings(tokens)

	return strings.Join(tokens, ",")
}
