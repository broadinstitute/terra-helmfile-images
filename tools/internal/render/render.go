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

/* Name of the `helmfile` binary */
const helmfileCommand = "helmfile"

/* Subdirectory to search for environment config files */
const envSubdir = "environments"

/* Struct encapsulating options for a render */
type Options struct {
	App            string
	Env            string
	ChartVersion   string
	AppVersion     string
	ChartDir       string
	OutputDir      string
	ArgocdMode     bool
	Stdout         bool
	Verbose        int
	ConfigRepoPath string
}

type Environment struct {
	name string
	base string
}

type Render struct {
	options          *Options               /* CLI options */
	environments     map[string]Environment /* Collection of environments defined in the config repo, keyed by env name */
	helmfileLogLevel string                 /* --log-level argument to pass to `helmfile` command */
}

/* ShellRunner object used to execute commands. Replaced with a mock in tests */
var shellRunner ShellRunner = &RealRunner{}

func NewRender(options *Options) (*Render, error) {
	render := new(Render)
	render.options = options

	environments, err := loadEnvironments(options.ConfigRepoPath)
	if err != nil {
		return nil, err
	}
	render.environments = environments

	render.helmfileLogLevel = "info"
	if options.Verbose > 1 {
		render.helmfileLogLevel = "debug"
	}

	return render, nil
}

/* Clean output directory */
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
		filePath := path.Join([]string{ r.options.OutputDir, file.Name() }...)
		log.Debug().Msgf("Deleting %s", filePath)

		err = os.RemoveAll(filePath)
		if err != nil {
			return err
		}
	}

	return nil
}

/* Update Helm repos */
func (r *Render) HelmUpdate() error {
	log.Debug().Msg("Updating Helm repos...")
	return r.runHelmfile("--allow-no-matching-release", "repos")
}

/* Render manifests */
func (r *Render) RenderAll() error {
	targetEnvs, err := r.getTargetEnvs()
	if err != nil {
		return err
	}

	log.Info().Msgf("Rendering manifests for %d environments...", len(targetEnvs))

	for _, env := range targetEnvs {
		err = r.renderSingleEnvironment(env)
		if err != nil {
			return err
		}
	}

	return normalizeRenderDirectories(r.options.OutputDir)
}

/* Scan through environments/ subdirectory and build a slice of defined environments */
func loadEnvironments(configRepoPath string) (map[string]Environment, error) {
	environments := make(map[string]Environment)

	envDir := path.Join(configRepoPath, envSubdir)
	if _, err := os.Stat(envDir); err != nil {
		return nil, fmt.Errorf("environments subdirectory %s does not exist in %s, is it a %s clone?", envSubdir, configRepoPath, ConfigRepoName)
	}

	matches, err := filepath.Glob(path.Join(envDir, "*", "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("error loading environments from %s: %v", configRepoPath, err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no environments found in %s, is it a %s clone?", configRepoPath, ConfigRepoName)
	}

	for _, filename := range matches {
		env := Environment{}
		env.base = path.Base(path.Dir(filename))
		env.name = strings.TrimSuffix(path.Base(filename), ".yaml")
		environments[env.name] = env
	}

	return environments, nil
}

func (r *Render) getTargetEnvs() ([]Environment, error) {
	if isSet(r.options.Env) {
		// User wants to render for a specific environment
		env, ok := r.environments[r.options.Env]
		if !ok {
			return nil, fmt.Errorf("unknown environment: %s", r.options.Env)
		}
		return []Environment{env}, nil
	}

	// No environment specified; render for _all_ environments
	var envs []Environment
	for _, env := range r.environments {
		envs = append(envs, env)
	}

	// Sort environments so they are rendered in a predictable order
	sortEnvironments(envs)

	return envs, nil
}

func (r *Render) renderSingleEnvironment(env Environment) error {
	log.Info().Msgf("Rendering manifests for %s", env.name)

	var args []string

	// Append global Helmfile options
	args = append(args, "-e", env.name)

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
		/* Skip dependencies unless we're rendering a local chart, to save time */
		args = append(args, "--skip-deps")
	}

	if !r.options.Stdout {
		// Expand output dir to absolute path, because Helmfile assumes paths
		// are relative to helmfile.yaml and we want to be relative to CWD
		// filepath.Abs()
		args = append(args, fmt.Sprintf("--output-dir=%s/%s", r.options.OutputDir, env.name))
	}

	return r.runHelmfile(args...)
}

func (r *Render) runHelmfile(args ...string) error {
	extraArgs := []string{
		fmt.Sprintf("--log-level=%s", r.helmfileLogLevel),
	}
	args = append(extraArgs, args...)

	cmd := Command{}
	cmd.Prog = helmfileCommand
	cmd.Args = args
	cmd.Dir = r.options.ConfigRepoPath

	return shellRunner.Run(cmd)
}

/* Return map of state values that should be set on the command-line, given user-supplied options */
func (r *Render) getStateValues() map[string]string {
	stateValues := make(map[string]string)

	if isSet(r.options.ChartDir) {
		key := fmt.Sprintf("releases.%s.repo", r.options.App)
		stateValues[key] = r.options.ChartDir
	} else if isSet(r.options.ChartVersion) {
		key := fmt.Sprintf("releases.%s.chartVersion", r.options.App)
		stateValues[key] = r.options.ChartVersion
	}

	if isSet(r.options.AppVersion) {
		key := fmt.Sprintf("releases.%s.appVersion", r.options.App)
		stateValues[key] = r.options.AppVersion
	}

	return stateValues
}

/* Return map of Helmfile selectors that should be set on the command-line, given user-supplied options */
func (r *Render) getSelectors() map[string]string {
	selectors := make(map[string]string)
	selectors["group"] = "terra"
	if r.options.ArgocdMode {
		// Render ArgoCD manifests instead of application manifests
		selectors["group"] = "argocd"
	}

	if isSet(r.options.App) {
		// Render manifests for the given app only
		selectors["app"] = r.options.App
	}

	return selectors
}

/*
Convert auto-generated template directory names like
  helmfile-b47efc70-workspacemanager
into
  workspacemanager
so that diff -r can be run on two sets of rendered templates.

We enforce that all release names in an environment are unique,
so this should not cause conflicts.
*/
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

/*
Sort environments lexicographically by base, and then by name
*/
func sortEnvironments(envs []Environment) {
	sort.Slice(envs, func(i int, j int) bool {
		if envs[i].base == envs[j].base {
			return envs[i].name < envs[j].name
		} else {
			return envs[i].base < envs[j].base
		}
	})
}

/*
Join map[string]string to string containing comma-separated key-value pairs.
Eg. { "a": "b", "c": "d" } -> "a=b,c=d"
*/
func joinKeyValuePairs(pairs map[string]string) string {
	var tokens []string
	for k, v := range pairs {
		tokens = append(tokens, fmt.Sprintf("%s=%s", k, v))
	}

	// Sort tokens so they are always supplied in predictable order
	sort.Strings(tokens)

	return strings.Join(tokens, ",")
}
