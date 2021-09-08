package render

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// helmfileCommand is the name of the `helmfile` binary
const helmfileCommand = "helmfile"

// targetTypeEnvVar is the name of the environment variable used to pass target type to helmfile
const targetTypeEnvVar = "TARGET_TYPE"

// targetBaseEnvVar is the name of the environment variable used to pass target base to helmfile
const targetBaseEnvVar = "TARGET_BASE"

// targetNameEnvVar is the name of the environment variable used to pass target name to helmfile
const targetNameEnvVar = "TARGET_NAME"

// Options encapsulates CLI options for a render
type Options struct {
	Env            string   // If supplied, render for a single environment instead of all targets
	Cluster        string   // If supplied, render for a single cluster instead of all targets
	Release        string   // If supplied, render for specific release only instead of all releases in environment/cluster
	ChartVersion   string   // Optionally override chart version
	AppVersion     string   // Optionally override app version
	ChartDir       string   // When supplied, render chart from local directory instead of released version
	ValuesFiles    []string // Optionally list of files for overriding chart values
	ArgocdMode     bool     // If true, render ArgoCD manifests instead of application manifests
	OutputDir      string   // Render to custom output directory intead of terra-helmfile/output
	Stdout         bool     // Render to stdout instead of output directory
	Verbose        int      // Invoke `helmfile` with verbose logging
	ConfigRepoPath string   // Path to terra-helmfile repo clone
}

// Render generates manifests by invoking `helmfile` with the appropriate arguments
type Render struct {
	options          *Options                 // CLI options
	environments     map[string]ReleaseTarget // Collection of environments defined in the config repo, keyed by env name
	clusters         map[string]ReleaseTarget // Collection of clusters defined in the config repo, keyed by cluster name
	helmfileLogLevel string                   // --log-level argument to pass to `helmfile` command
}

// Global/package-level variable, used for executing commands. Replaced with a mock in tests.
var shellRunner ShellRunner = &RealRunner{}

// NewRender is a constructor
func NewRender(options *Options) (*Render, error) {
	render := new(Render)
	render.options = options

	environments, err := loadEnvironments(options.ConfigRepoPath)
	if err != nil {
		return nil, err
	}
	render.environments = environments

	clusters, err := loadClusters(options.ConfigRepoPath)
	if err != nil {
		return nil, err
	}
	render.clusters = clusters

	render.helmfileLogLevel = "info"
	if options.Verbose > 1 {
		render.helmfileLogLevel = "debug"
	}

	return render, nil
}

// CleanOutputDirectory removes any old files from output directory
func (r *Render) CleanOutputDirectory() error {
	if r.options.Stdout {
		// No need to clean output directory if we're rendering to stdout
		return nil
	}

	if _, err := os.Stat(r.options.OutputDir); os.IsNotExist(err) {
		// Output dir does not exist, nothing to clean up
		return nil
	}

	// Delete any files that exist inside the output directory.
	log.Debug().Msgf("Cleaning output directory: %s", r.options.OutputDir)

	// This code would be simpler if we could just call os.RemoveAll() on the
	// output directory itself, but in some cases the output directory is a volume
	// mount, and trying to remove it throws an error.
	dir, err := ioutil.ReadDir(r.options.OutputDir)
	if err != nil {
		return err
	}

	for _, file := range dir {
		filePath := path.Join([]string{r.options.OutputDir, file.Name()}...)
		log.Debug().Msgf("Deleting %s", filePath)

		err = os.RemoveAll(filePath)
		if err != nil {
			return err
		}
	}

	return nil
}

// HelmUpdate updates Helm repos
func (r *Render) HelmUpdate() error {
	log.Debug().Msg("Updating Helm repos...")
	return r.runHelmfile([]string{}, "--allow-no-matching-release", "repos")
}

// RenderAll renders manifests based on supplied arguments
func (r *Render) RenderAll() error {
	targetEnvs, err := r.getTargets()
	if err != nil {
		return err
	}

	log.Info().Msgf("Rendering manifests for %d environments...", len(targetEnvs))

	for _, env := range targetEnvs {
		err = r.renderSingleReleaseTarget(env)
		if err != nil {
			return err
		}
	}

	return normalizeRenderDirectories(r.options.OutputDir)
}

// loadEnvironments scans through the environments/ subdirectory and build a slice of defined environments
func loadEnvironments(configRepoPath string) (map[string]ReleaseTarget, error) {
	configDir := path.Join(configRepoPath, envConfigDir)

	return loadTargets(configDir, envTypeName, NewEnvironmentGeneric)
}

// loadClusters scans through the cluster/ subdirectory and build a slice of defined clusters
func loadClusters(configRepoPath string) (map[string]ReleaseTarget, error) {
	configDir := path.Join(configRepoPath, clusterConfigDir)

	return loadTargets(configDir, clusterTypeName, NewClusterGeneric)
}

// loadTargets loads the set of configured clusters or environments from a config directory
func loadTargets(configDir string, targetType string, constructor func(string, string) ReleaseTarget) (map[string]ReleaseTarget, error) {
	targets := make(map[string]ReleaseTarget)

	if _, err := os.Stat(configDir); err != nil {
		return nil, fmt.Errorf("%s directory %s does not exist, is it a %s clone?", targetType, configDir, configRepoName)
	}

	matches, err := filepath.Glob(path.Join(configDir, "*", "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("error loading %s configs from %s: %v", targetType, configDir, err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no %s configs found in %s, is it a %s clone?", targetType, configDir, configRepoName)
	}

	for _, filename := range matches {
		base := path.Base(path.Dir(filename))
		name := strings.TrimSuffix(path.Base(filename), ".yaml")

		target := constructor(name, base)
		targets[target.Name()] = target
	}

	return targets, nil
}

// getTargets returns the set of release targets to render manifests for
func (r *Render) getTargets() ([]ReleaseTarget, error) {
	if isSet(r.options.Env) {
		// User wants to render for a specific environment
		env, ok := r.environments[r.options.Env]
		if !ok {
			return nil, fmt.Errorf("unknown environment: %s", r.options.Env)
		}
		return []ReleaseTarget{env}, nil
	}

	if isSet(r.options.Cluster) {
		// User wants to render for a specific cluster
		cluster, ok := r.clusters[r.options.Cluster]
		if !ok {
			return nil, fmt.Errorf("unknown cluster: %s", r.options.Cluster)
		}
		return []ReleaseTarget{cluster}, nil
	}

	// No target specified; render for _all_ targets
	var targets []ReleaseTarget
	for _, env := range r.environments {
		targets = append(targets, env)
	}
	for _, cluster := range r.clusters {
		targets = append(targets, cluster)
	}

	// Sort targets so they are rendered in a predictable order
	sortReleaseTargets(targets)

	return targets, nil
}

func (r *Render) renderSingleReleaseTarget(target ReleaseTarget) error {
	log.Info().Msgf("Rendering manifests for %s %s", target.Type(), target.Name())

	// Get environment variables for `helmfile`
	envVars := getEnvVars(target)

	// Build list of CLI args to `helmfile`
	var args []string

	selectors := r.getSelectors()
	if len(selectors) != 0 {
		args = append(args, fmt.Sprintf("--selector=%s", joinKeyValuePairs(selectors)))
	}

	stateValues := r.getStateValues()
	if len(stateValues) != 0 {
		args = append(args, fmt.Sprintf("--state-values-set=%s", joinKeyValuePairs(stateValues)))
	}

	// Append Helmfile command we're running (template)
	args = append(args, "template")

	// Append arguments specific to template subcommand
	if r.options.ChartDir == optionUnset {
		// Skip dependencies unless we're rendering a local chart, to save time
		args = append(args, "--skip-deps")
	}

	if len(r.options.ValuesFiles) > 0 {
		args = append(args, fmt.Sprintf("--values=%s", strings.Join(r.options.ValuesFiles, ",")))
	}

	if !r.options.Stdout {
		// Expand output dir to absolute path, because Helmfile assumes paths
		// are relative to helmfile.yaml and we want to be relative to CWD
		// filepath.Abs()
		args = append(args, fmt.Sprintf("--output-dir=%s/%s", r.options.OutputDir, target.Name()))
	}

	return r.runHelmfile(envVars, args...)
}

func (r *Render) runHelmfile(envVars []string, args ...string) error {
	extraArgs := []string{
		fmt.Sprintf("--log-level=%s", r.helmfileLogLevel),
	}
	args = append(extraArgs, args...)

	cmd := Command{}
	cmd.Env = envVars
	cmd.Prog = helmfileCommand
	cmd.Args = args
	cmd.Dir = r.options.ConfigRepoPath

	return shellRunner.Run(cmd)
}

// getEnvVars returns a slice of environment variables that are needed for a helmfile render, . Eg.
// []string{"TARGET_TYPE": "environment", "TARGET_NAME": "dev", "TARGET_BASE": "live" }
func getEnvVars(t ReleaseTarget) []string {
	m := map[string]string{
		targetTypeEnvVar: t.Type(),
		targetNameEnvVar: t.Name(),
		targetBaseEnvVar: t.Base(),
	}

	var pairs []string
	for name, value := range m {
		pairs = append(pairs, fmt.Sprintf("%s=%s", name, value))
	}
	return pairs
}

// getStateValues returns a map of state values that should be set on the command-line, given user-supplied options
func (r *Render) getStateValues() map[string]string {
	stateValues := make(map[string]string)

	if isSet(r.options.ChartDir) {
		key := fmt.Sprintf("releases.%s.repo", r.options.Release)
		stateValues[key] = r.options.ChartDir
	} else if isSet(r.options.ChartVersion) {
		key := fmt.Sprintf("releases.%s.chartVersion", r.options.Release)
		stateValues[key] = r.options.ChartVersion
	}

	if isSet(r.options.AppVersion) {
		key := fmt.Sprintf("releases.%s.appVersion", r.options.Release)
		stateValues[key] = r.options.AppVersion
	}

	return stateValues
}

// getSelectors returns a map of Helmfile selectors that should be set on the command-line, given user-supplied options
func (r *Render) getSelectors() map[string]string {
	selectors := make(map[string]string)
	selectors["manifestType"] = "release"
	if r.options.ArgocdMode {
		// Render ArgoCD manifests instead of application manifests
		selectors["manifestType"] = "argocd"
	}

	if isSet(r.options.Release) {
		// Render manifests for the given app only
		selectors["release"] = r.options.Release
	}

	return selectors
}

// normalizeRenderDirectories converts auto-generated template directory names like
//   helmfile-b47efc70-workspacemanager
// into
//   workspacemanager
// so that diff -r can be run on two sets of rendered templates.
//
// We enforce that all release names in an environment are unique,
// so this should not cause conflicts.
func normalizeRenderDirectories(outputDir string) error {
	matches, err := filepath.Glob(path.Join(outputDir, "*", "helmfile-*"))
	if err != nil {
		return fmt.Errorf("error normalizing render directories in %s: %v", outputDir, err)
	}

	for _, oldPath := range matches {
		baseName := path.Base(oldPath) // Eg. "helmfile-b47efc70-workspacemanager
		parent := path.Dir(oldPath)

		re := regexp.MustCompile("^helmfile-[^-]+-")
		newName := re.ReplaceAllString(baseName, "")
		newPath := path.Join(parent, newName)

		log.Debug().Msgf("Renaming %s to %s", oldPath, newPath)
		if err = os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("error renaming render directory %s to %s: %v", oldPath, newPath, err)
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
