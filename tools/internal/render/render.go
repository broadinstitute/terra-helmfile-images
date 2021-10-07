package render

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
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

// Environment variables -- prefixed with THF for "Terra Helmfile"
//
// helmfileTargetTypeEnvVar is the name of the environment variable used to pass target type to helmfile
const helmfileTargetTypeEnvVar = "THF_TARGET_TYPE"

// hemfileTargetBaseEnvVar is the name of the environment variable used to pass target base to helmfile
const hemfileTargetBaseEnvVar = "THF_TARGET_BASE"

// helmfileTargetNameEnvVar is the name of the environment variable used to pass target name to helmfile
const helmfileTargetNameEnvVar = "THF_TARGET_NAME"

// helmfileScratchDirEnvVar is the name of the environment variable used to pass unpack dir to helmfile
const helmfileScratchDirEnvVar = "THF_SCRATCH_DIR"

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
	ScratchDir     string   // Supply a specific scratch directory to Helmfile instead of creating & deleting a tmp dir
}

// render generates manifests by invoking `helmfile` with the appropriate arguments
type render struct {
	options          Options                  // CLI options
	scratchDir       scratchDir               // Scratch directory where charts may be downloaded and unpacked, etc.
	releaseTargets   map[string]ReleaseTarget // Collection of release targets (environments and clusters) defined in the config repo, keyed by name
	helmfileLogLevel string                   // --log-level argument to pass to `helmfile` command
}

// scratchDir struct containing paths for temporary/scratch work
type scratchDir struct {
	root              string // root directory for all scratch files
	cleanupOnTeardown bool   // cleanUpOnExit if true, scratch files will be deleted on exit
	helmfileDir       string // scratch directory that helmfile.yaml should use
}

// Global/package-level variable, used for executing commands. Replaced with a mock in tests.
var shellRunner shell.Runner = shell.NewRealRunner()

// DoRender constructs a render and invokes all functions in correct order to perform a complete
// render.
func DoRender(options Options) error {
	render, err := newRender(options)
	if err != nil {
		return err
	}
	if err = render.Setup(); err != nil {
		return err
	}
	if err = render.HelmUpdate(); err != nil {
		return err
	}
	if err = render.RenderAll(); err != nil {
		return err
	}
	if err = render.Teardown(); err != nil {
		return err
	}
	return nil
}

// newRender is a constructor
func newRender(options Options) (*render, error) {
	render := new(render)
	render.options = options

	targets, err := loadTargets(options.ConfigRepoPath)
	if err != nil {
		return nil, err
	}
	render.releaseTargets = targets

	render.helmfileLogLevel = "info"
	if options.Verbose > 1 {
		render.helmfileLogLevel = "debug"
	}

	scratchDir, err := createScratchDir(options)
	if err != nil {
		return nil, err
	}
	render.scratchDir = *scratchDir

	return render, nil
}

// Setup performs necessary setup for a render
func (r *render) Setup() error {
	return r.cleanOutputDirectory()
}

// Teardown cleans up resources, such as the Helmfile scratch directory, that are no longer needed
// once the render is finished
func (r *render) Teardown() error {
	if r.scratchDir.cleanupOnTeardown {
		return os.RemoveAll(r.scratchDir.root)
	}
	return nil
}

// HelmUpdate updates Helm repos
func (r *render) HelmUpdate() error {
	log.Debug().Msg("Updating Helm repos...")
	return r.runHelmfile([]string{}, "--allow-no-matching-release", "repos")
}

// RenderAll renders manifests based on supplied arguments
func (r *render) RenderAll() error {
	targetEnvs, err := r.getTargets()
	if err != nil {
		return err
	}

	log.Info().Msgf("Rendering manifests for %d environments...", len(targetEnvs))

	for _, env := range targetEnvs {
		err = r.renderSingleTarget(env)
		if err != nil {
			return err
		}
	}

	return normalizeRenderDirectories(r.options.OutputDir)
}

// createScratchDir creates scratch directory structure, given user-supplied options
func createScratchDir(options Options) (*scratchDir, error) {
	scratchDir := scratchDir{}
	if isSet(options.ScratchDir) {
		// User supplied a scratch directory
		scratchDir.root = options.ScratchDir
		scratchDir.cleanupOnTeardown = false
	} else {
		root, err := os.MkdirTemp("", "render-unpack-*")
		if err != nil {
			return nil, err
		}
		log.Debug().Msgf("Created new scratch directory: %s", root)
		scratchDir.root = root
		scratchDir.cleanupOnTeardown = true
	}

	scratchDir.helmfileDir = path.Join(scratchDir.root, "helmfile")

	dirs := []string{ scratchDir.helmfileDir }
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	return &scratchDir, nil
}

// cleanOutputDirectory removes any old files from output directory
func (r *render) cleanOutputDirectory() error {
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

// loadTargets loads all environment and cluster release targets from the config repo
func loadTargets(configRepoPath string) (map[string]ReleaseTarget, error) {
	targets := make(map[string]ReleaseTarget)

	environments, err := loadEnvironments(configRepoPath)
	if err != nil {
		return nil, err
	}

	clusters, err := loadClusters(configRepoPath)
	if err != nil {
		return nil, err
	}

	for key, env := range environments {
		targets[key] = env
	}

	for key, cluster := range clusters {
		if conflict, ok := targets[key]; ok {
			return nil, fmt.Errorf("cluster name %s conflicts with environment name %s", cluster.Name(), conflict.Name())
		}
		targets[key] = cluster
	}

	return targets, nil
}

// loadEnvironments scans through the environments/ subdirectory and build a slice of defined environments
func loadEnvironments(configRepoPath string) (map[string]ReleaseTarget, error) {
	configDir := path.Join(configRepoPath, envConfigDir)

	return loadTargetsFromDirectory(configDir, envTypeName, NewEnvironmentGeneric)
}

// loadClusters scans through the cluster/ subdirectory and build a slice of defined clusters
func loadClusters(configRepoPath string) (map[string]ReleaseTarget, error) {
	configDir := path.Join(configRepoPath, clusterConfigDir)

	return loadTargetsFromDirectory(configDir, clusterTypeName, NewClusterGeneric)
}

// loadTargetsFromDirectory loads the set of configured clusters or environments from a config directory
func loadTargetsFromDirectory(configDir string, targetType string, constructor func(string, string) ReleaseTarget) (map[string]ReleaseTarget, error) {
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

		if conflict, ok := targets[target.Name()]; ok {
			return nil, fmt.Errorf("%s name conflict %s (%s) and %s (%s)", targetType, target.Name(), target.Base(), conflict.Name(), conflict.Base())
		}

		targets[target.Name()] = target
	}

	return targets, nil
}

// getTargets returns the set of release targets to render manifests for
func (r *render) getTargets() ([]ReleaseTarget, error) {

	if isSet(r.options.Env) {
		// User wants to render for a specific environment
		env, ok := r.releaseTargets[r.options.Env]
		if !ok {
			return nil, fmt.Errorf("unknown environment: %s", r.options.Env)
		}
		return []ReleaseTarget{env}, nil
	}

	if isSet(r.options.Cluster) {
		// User wants to render for a specific cluster
		cluster, ok := r.releaseTargets[r.options.Cluster]
		if !ok {
			return nil, fmt.Errorf("unknown cluster: %s", r.options.Cluster)
		}
		return []ReleaseTarget{cluster}, nil
	}

	// No target specified; render for _all_ targets
	var targets []ReleaseTarget
	for _, target := range r.releaseTargets {
		targets = append(targets, target)
	}

	// Sort targets so they are rendered in a predictable order
	sortReleaseTargets(targets)

	return targets, nil
}

// renderSingleTarget renders manifests for a single environment or cluster
func (r *render) renderSingleTarget(target ReleaseTarget) error {
	log.Info().Msgf("Rendering manifests for %s %s", target.Type(), target.Name())

	// Get environment variables for `helmfile`
	envVars := r.getEnvVarsForTarget(target)

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

func (r *render) runHelmfile(envVars []string, args ...string) error {
	finalArgs := []string{
		fmt.Sprintf("--log-level=%s", r.helmfileLogLevel),
	}
	finalArgs = append(finalArgs, args...)

	cmd := shell.Command{}
	cmd.Env = envVars
	cmd.Prog = helmfileCommand
	cmd.Args = finalArgs
	cmd.Dir = r.options.ConfigRepoPath

	return shellRunner.Run(cmd)
}

// getEnvVarsForTarget returns a slice of environment variables that are needed for a helmfile render, . Eg.
// []string{"TARGET_TYPE": "environment", "TARGET_NAME": "dev", "TARGET_BASE": "live" }
func (r *render) getEnvVarsForTarget(t ReleaseTarget) []string {
	return []string{
		fmt.Sprintf("%s=%s", helmfileTargetTypeEnvVar, t.Type()),
		fmt.Sprintf("%s=%s", hemfileTargetBaseEnvVar, t.Base()),
		fmt.Sprintf("%s=%s", helmfileTargetNameEnvVar, t.Name()),
		fmt.Sprintf("%s=%s", helmfileScratchDirEnvVar, r.scratchDir.helmfileDir),
	}
}

// getStateValues returns a map of state values that should be set on the command-line, given user-supplied options
func (r *render) getStateValues() map[string]string {
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
func (r *render) getSelectors() map[string]string {
	selectors := make(map[string]string)
	selectors["mode"] = "release"
	if r.options.ArgocdMode {
		// Render ArgoCD manifests instead of application manifests
		selectors["mode"] = "argocd"
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
