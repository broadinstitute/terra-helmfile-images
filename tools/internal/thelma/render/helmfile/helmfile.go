package helmfile

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/target"
	"github.com/rs/zerolog/log"
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
	ReleaseName  *string  // ReleaseName optionally render for specific release only instead of all releases in target environment/cluster
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
	ShellRunner      shell.Runner // ShellRunner shell Runner to use for executing helmfile commands
}

// ConfigRepo can be used to `helmfile` render commands on a clone of the `terra-helmfile` repo
type ConfigRepo struct {
	path             string
	chartCacheDir    string
	helmfileLogLevel string
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

// HelmUpdate updates Helm repos
func (r *ConfigRepo) HelmUpdate() error {
	log.Debug().Msg("Updating Helm repos...")
	return r.runHelmfile([]string{}, "--allow-no-matching-release", "repos")
}

// RenderToStdout renders to stdout
func (r *ConfigRepo) RenderToStdout(target target.ReleaseTarget, helmfileArgs *Args) error {
	return r.renderSingleTarget(target, helmfileArgs)
}

// RenderToDir renders manfiests into a target directory (the directory will be created if it does not exist)
func (r *ConfigRepo) RenderToDir(target target.ReleaseTarget, outputDir string, helmfileArgs *Args) error {
	outputDirFlag := fmt.Sprintf("--output-dir=%s", outputDir)

	if err := r.renderSingleTarget(target, helmfileArgs, outputDirFlag); err != nil {
		return err
	}

	return normalizeOutputDir(outputDir)
}

// renderSingleTarget renders manifests for a single environment or cluster
func (r *ConfigRepo) renderSingleTarget(target target.ReleaseTarget, args *Args, extraArgs ...string) error {
	log.Info().Msgf("Rendering manifests for %s %s", target.Type(), target.Name())

	// Take given helmfileArgs and translate them to helmfile parameters (selectors, state values & env vars)
	params := newHelmfileParams()
	params.setTargetEnvVars(target)
	params.setChartCacheDirEnvVar(r.chartCacheDir)
	params.applySelectorSettings(args)
	params.applyVersionSettings(args)

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

	cliArgs = append(cliArgs, extraArgs...)

	return r.runHelmfile(params.envVars, cliArgs...)
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
func (params *helmfileParams) setTargetEnvVars(t target.ReleaseTarget) {
	params.addEnvVar(TargetTypeEnvVar, t.Type())
	params.addEnvVar(TargetBaseEnvVar, t.Base())
	params.addEnvVar(TargetNameEnvVar, t.Name())
}

// setChartCacheDirEnvVar sets the chart cache dir env var
func (params *helmfileParams) setChartCacheDirEnvVar(chartCacheDir string) {
	params.addEnvVar(ChartCacheDirEnvVar, chartCacheDir)
}

// applyVersionSettings updates params with necessary state values & environment variables, based on
// the configured chart version, chart dir, & app version
func (params *helmfileParams) applyVersionSettings(helmfileArgs *Args) {
	if helmfileArgs.ChartDir != nil && helmfileArgs.ReleaseName != nil {
		params.addEnvVar(ChartSrcDirEnvVar, *helmfileArgs.ChartDir)

		key := fmt.Sprintf("releases.%s.chartVersion", *helmfileArgs.ReleaseName)
		params.stateValues[key] = localChartVersion
	} else if helmfileArgs.ChartVersion != nil && helmfileArgs.ReleaseName != nil {
		key := fmt.Sprintf("releases.%s.chartVersion", *helmfileArgs.ReleaseName)
		params.stateValues[key] = *helmfileArgs.ChartVersion
	}

	if helmfileArgs.AppVersion != nil && helmfileArgs.ReleaseName != nil {
		key := fmt.Sprintf("releases.%s.appVersion", *helmfileArgs.ReleaseName)
		params.stateValues[key] = *helmfileArgs.AppVersion
	}
}

// applySelectorSettings populates selectors with correct values based on the ArgocdMode and Release arguments.
func (params *helmfileParams) applySelectorSettings(helmfileArgs *Args) {
	params.selectors["mode"] = "release"
	if helmfileArgs.ArgocdMode {
		// Render ArgoCD manifests instead of application manifests
		params.selectors["mode"] = "argocd"
	}

	if helmfileArgs.ReleaseName != nil {
		// Render manifests for the given app only
		params.selectors["release"] = *helmfileArgs.ReleaseName
	}
}

// addEnvVar adds an env var key/value pair to the given params instance
func (params *helmfileParams) addEnvVar(name string, value string) {
	params.envVars = append(params.envVars, fmt.Sprintf("%s=%s", name, value))
}

// normalizeOutputDir removes "helmfile-.*" directories from helmfile output paths.
// this makes it possible to easily run diff -r on render outputs from different branches
func normalizeOutputDir(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	glob := path.Join(outputDir, "helmfile-*")
	matches, err := filepath.Glob(glob)
	if err != nil {
		return fmt.Errorf("error globbing rendered templates %s: %v", glob, err)
	}

	for _, match := range matches {
		releaseName, err := extractReleaseName(match)
		if err != nil {
			return err
		}
		dest := path.Join(outputDir, releaseName)
		log.Debug().Msgf("Renaming %s to %s", match, dest)
		if err := os.Rename(match, dest); err != nil {
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

// extractReleaseName given a path to helmfile output dir, return release name component
// eg. extractReleaseName("helmfile-b47efc70-cromiam") -> "cromiam"
func extractReleaseName(helmfileOutputDir string) (string, error) {
	base := path.Base(helmfileOutputDir)
	tokens := strings.SplitN(base, "-", 3)
	if len(tokens) != 3 {
		return "", fmt.Errorf("expected helmfile output dir in form helmfile-<id>-<releasename>, got %s", base)
	}
	return tokens[2], nil
}
