package cli

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/render"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/render/helmfile"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"os"
	"path"
	"path/filepath"
)

// This file handles CLI option parsing the render utility. It uses Cobra in accordance with the
// documentation here: https://github.com/spf13/cobra/blob/master/user_guide.md

const helpMessage = `Renders Terra Kubernetes manifests

Examples:

# Render all manifests for all Terra services in all environments
render

# Render manifests for all Terra services in the dev environment
render -e dev

# Render manifests for the cromwell service in the alpha environment
render -e alpha -a cromwell

# Render manifests for the cromwell service in the alpha environment,
# overriding app and chart version
render -e alpha -a cromwell --chart-version="~> 0.8" --app-version="53-9b11416"

# Render manifests from a local copy of a chart
render -e alpha -a cromwell --chart-dir=../terra-helm/charts

# Render manifests, overriding chart values with a local file
render -e alpha -a cromwell --values-file=path/to/my-values.yaml

# Render all manifests to a directory called my-manifests
render --output-dir=/tmp/my-manifests

# Render ArgoCD manifests for all Terra services in all environments
render --argocd

# Render ArgoCD manifests for the Cromwell service in the alpha environment
render -e alpha -a cromwell --argocd
`

// configRepoName name of the Helmfile configuration repo
const configRepoName = "terra-helmfile"

// configRepoPathEnvVar Environment variable used to set path to config repo clone
const configRepoPathEnvVar = "TERRA_HELMFILE_PATH"

// defaultOutputDirName name of default output directory
const defaultOutputDirName = "output"

// CLI captures necessary state and configuration for executing a render from the command-line
type CLI struct {
	cobraCommand *cobra.Command
	helmfileArgs *helmfile.Args
	renderOptions *render.Options
	flagVals *flagValues
	envVarOverrides map[string]string
	noop bool
}

// Names of all the CLI flags are kept in a struct so they can be easily referenced in error messages
var flagNames = struct {
	env             string
	cluster         string
	app             string
	release         string
	chartDir        string
	chartVersion    string
	appVersion      string
	valuesFile      string
	argocd          string
	outputDir       string
	scratchDir      string
	stdout          string
	verbosity       string
}{
	env:            "env",
	cluster:        "cluster",
	app:            "app",
	release:         "release",
	chartDir:        "chart-dir",
	chartVersion:    "chart-version",
	appVersion:      "app-version",
	valuesFile:      "values-file",
	argocd:          "argocd",
	outputDir:       "output-dir",
	scratchDir:      "scratch-dir",
	stdout:          "stdout",
	verbosity:       "verbosity",
}

// flagValues is a struct for capturing flag values that are parsed by Cobra.
type flagValues struct {
	env          string
	cluster      string
	app          string
	release         string
	chartDir        string
	chartVersion    string
	appVersion      string
	valuesFile      []string
	argocd          bool
	outputDir       string
	scratchDir      string
	stdout          bool
	verbosity       int
}

// Execute is the main method/entrypoint for the render tool.
func Execute() {
	cli := newCLI(false)

	if err := cli.execute(); err != nil {
		log.Error().Msgf("%v", err)
		os.Exit(1)
	}
}

// execute executes a CLI command
func (cli *CLI) execute() error {
	return cli.cobraCommand.Execute()
}

// setArgs sets cli args on the cobra command. Used in testing.
func (cli *CLI) setArgs(args []string) {
	cli.cobraCommand.SetArgs(args)
}

// setEnvVar overrides process environment variables for this CLI instance. Used in testing.
func (cli *CLI) setEnvVar(name string, value string) {
	cli.envVarOverrides[name] = value
}

// unsetEnvVar removes an environment variable override for this CLI instance. Used in testing.
func (cli *CLI) unsetEnvVar(name string) {
	delete(cli.envVarOverrides, name)
}

// newCLI is a package-private constructor for CLI objects that exposes more configuration options than the
// public constructor.
//
// set `noop` to true to construct a CLI object that will parse arguments but not actually do anything
// when Execute() is called
func newCLI(noop bool) *CLI {
	flagVals := &flagValues{}
	helmfileArgs := &helmfile.Args{}
	renderOptions := &render.Options{}

	cobraCommand := &cobra.Command{
		Use:   "render [options]",
		Short: "Renders Terra Kubernetes manifests",
		Long: helpMessage,
		SilenceUsage: true, // Only print out usage error when user supplies -h/--help
		SilenceErrors: true, // Don't print errors, we do it ourselves using a logging library
	}

	// Add CLI flag handlers to cobra command
	cobraCommand.Flags().StringVarP(&flagVals.env, flagNames.env, "e", "ENV", "Render manifests for a specific Terra environment only")
	cobraCommand.Flags().StringVarP(&flagVals.cluster, flagNames.cluster, "c", "CLUSTER", "Render manifests for a specific Terra cluster only")
	cobraCommand.Flags().StringVarP(&flagVals.release, flagNames.release, "r", "RELEASE", "Render manifests for a specific release only")
	cobraCommand.Flags().StringVarP(&flagVals.app, flagNames.app, "a", "APP", "Render manifests for a specific app only. (Alias for -r/--release)")
	cobraCommand.Flags().StringVar(&flagVals.chartVersion, flagNames.chartVersion, "VERSION", "Override chart version")
	cobraCommand.Flags().StringVar(&flagVals.chartDir, flagNames.chartDir, "path/to/charts", "Render from local chart directory instead of official release")
	cobraCommand.Flags().StringVar(&flagVals.appVersion, flagNames.appVersion, "VERSION", "Override application version")
	cobraCommand.Flags().StringSliceVar(&flagVals.valuesFile, flagNames.valuesFile, []string{}, "Path to chart values file. Can be specified multiple times with ascending precedence (last wins)")
	cobraCommand.Flags().BoolVar(&flagVals.argocd, flagNames.argocd, false, "Render ArgoCD manifests instead of application manifests")
	cobraCommand.Flags().StringVarP(&flagVals.outputDir, flagNames.outputDir, "d", "path/to/output/dir", "Render manifests to custom output directory")
	cobraCommand.Flags().StringVar(&flagVals.scratchDir, flagNames.scratchDir, "path/to/scratch/dir", "Use a pre-defined scratch directory instead of creating and deleting a tmp dir (useful for debugging")
	cobraCommand.Flags().BoolVar(&flagVals.stdout, flagNames.stdout, false, "Render manifests to stdout instead of output directory")
	cobraCommand.Flags().CountVarP(&flagVals.verbosity, flagNames.verbosity, "v", "Verbose logging. Can be specified multiple times")

	cli := &CLI{
		cobraCommand: cobraCommand,
		renderOptions: renderOptions,
		helmfileArgs: helmfileArgs,
		flagVals: flagVals,
		envVarOverrides: make(map[string]string),
		noop: noop,
	}

	// Configure main body of the cobra command. I sort of hate this mutual reference
	// (cli object holds the cobra command, the cobra command now has a hook that references the cli)
	// but it makes the argument processing code much simpler
	cobraCommand.RunE = func(_ *cobra.Command, args []string) error {
		return cli.runE(args)
	}

	return cli
}

func init() {
	// Initialize logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

// runE should be called from the cobra command's RunE hook
func (cli *CLI) runE(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("expected no positional arguments, got %v", args)
	}
	if err := cli.handleAliases(); err != nil {
		return err
	}
	if err := cli.checkIncompatibleFlags(); err != nil {
		return err
	}
	if err := cli.fillRenderOptions(); err != nil {
		return err
	}
	if err := cli.fillHelmfileArgs(); err != nil {
		return err
	}

	if cli.noop {
		// noop-mode! execute after parsing and validating arguments but before actually
		// doing anything
		return nil
	}

	adjustLoggingVerbosity(cli.renderOptions.Verbosity)

	return render.DoRender(cli.renderOptions, cli.helmfileArgs)
}

// fillRenderOptions populates an empty render.Options struct in accordance with user-supplied CLI options
func (cli *CLI) fillRenderOptions() error {
	flags := cli.cobraCommand.Flags()
	flagVals := cli.flagVals
	renderOptions := cli.renderOptions

	// config repo path (path to terra-helmfile clone)
	configRepoPath, defined := cli.lookupEnvVar(configRepoPathEnvVar)
	if !defined {
		return fmt.Errorf("please specify path to %s clone via the environment variable %s", configRepoName, configRepoPathEnvVar)
	}
	configRepoPath, err := expandAndVerifyExists(configRepoPath, "config repo clone")
	if err != nil {
		return err
	}
	renderOptions.ConfigRepoPath = configRepoPath

	// env
	if flags.Changed(flagNames.env) {
		renderOptions.Env = &flagVals.env
	}

	// cluster
	if flags.Changed(flagNames.cluster) {
		renderOptions.Cluster = &flagVals.cluster
	}

	// output dir
	if flags.Changed(flagNames.outputDir) {
		dir, err := filepath.Abs(flagVals.outputDir)
		if err != nil {
			return err
		}
		renderOptions.OutputDir = dir
	} else {
		renderOptions.OutputDir = path.Join(renderOptions.ConfigRepoPath, defaultOutputDirName)
		log.Debug().Msgf("Using default output dir %s", renderOptions.OutputDir)
	}

	// scratch dir
	if flags.Changed(flagNames.scratchDir) {
		dir, err := expandAndVerifyExists(flagVals.scratchDir, "scratch dir")
		if err != nil {
			return err
		}
		renderOptions.ScratchDir = &dir
	}

	// stdout
	renderOptions.Stdout = flagVals.stdout

	// verbosity
	renderOptions.Verbosity = flagVals.verbosity

	return nil
}

// fillHelmfileArgs populates an empty helfile.Args struct in accordance with user-supplied CLI options
func (cli *CLI) fillHelmfileArgs() error {
	flags := cli.cobraCommand.Flags()
	flagVals := cli.flagVals
	helmfileArgs := cli.helmfileArgs

	// release name
	if flags.Changed(flagNames.release) {
		helmfileArgs.ReleaseName = &flagVals.release
	}

	// chart version
	if flags.Changed(flagNames.chartVersion) {
		helmfileArgs.ChartVersion = &flagVals.chartVersion
	}

	// app version
	if flags.Changed(flagNames.appVersion) {
		helmfileArgs.AppVersion = &flagVals.appVersion
	}

	// chart dir
	if flags.Changed(flagNames.chartDir) {
		dir, err := expandAndVerifyExists(flagVals.chartDir, "chart dir")
		if err != nil {
			return err
		}
		helmfileArgs.ChartDir = &dir
	}

	// values file
	if flags.Changed(flagNames.valuesFile) {
		for _, file := range flagVals.valuesFile {
			fullpath, err := expandAndVerifyExists(file, "values file")
			if err != nil {
				return err
			}
			helmfileArgs.ValuesFiles = append(helmfileArgs.ValuesFiles, fullpath)
		}
	}

	// argocd mode
	helmfileArgs.ArgocdMode = flagVals.argocd

	return nil
}

// given a flagset, look for legacy aliases and update the new flag.
func (cli *CLI) handleAliases() error {
	flags := cli.cobraCommand.Flags()

	// --app is a legacy alias for --release, so copy the user-supplied value over
	if flags.Changed(flagNames.app) {
		if flags.Changed(flagNames.release) {
			return fmt.Errorf("-a is a legacy alias for -r, please specify one or the other but not both")
		}
		app := flags.Lookup(flagNames.app).Value.String()
		err := flags.Set(flagNames.release, app)
		if err != nil {
			return fmt.Errorf("error setting --%s to --%s argument %q: %v", flagNames.release, flagNames.app, app, err)
		}
	}

	return nil
}

func (cli *CLI) checkIncompatibleFlags() error {
	flags := cli.cobraCommand.Flags()

	if flags.Changed(flagNames.env) && flags.Changed(flagNames.cluster) {
		return fmt.Errorf("only one of --%s or --%s may be specified", flagNames.env, flagNames.cluster)
	}

	if flags.Changed(flagNames.release) && !(flags.Changed(flagNames.env) || flags.Changed(flagNames.cluster)) {
		// Not all targets include all charts, so require users to specify target env or cluster when -r / -a is passed in
		return fmt.Errorf("an environment (--%s) or cluster (--%s) must be specified when a release is specified with --%s", flagNames.env, flagNames.cluster, flagNames.release)
	}

	if flags.Changed(flagNames.chartDir) {
		if flags.Changed(flagNames.chartVersion) {
			// Chart dir points at a local chart copy, chart version specifies which version to use, we can only
			// use one or the other
			return fmt.Errorf("only one of --%s or --%s may be specified", flagNames.chartDir, flagNames.chartVersion)
		}

		if !flags.Changed(flagNames.release) {
			return fmt.Errorf("--%s requires a release be specified with --%s", flagNames.chartDir, flagNames.release)
		}
	}

	if flags.Changed(flagNames.chartVersion) && !flags.Changed(flagNames.release) {
		return fmt.Errorf("--%s requires a release be specified with --%s", flagNames.chartVersion, flagNames.release)
	}

	if flags.Changed(flagNames.appVersion) {
		if !flags.Changed(flagNames.release) {
			return fmt.Errorf("--%s requires a release be specified with --%s", flagNames.appVersion, flagNames.release)
		}
		if flags.Changed(flagNames.cluster) {
			return fmt.Errorf("--%s cannot be used for cluster releases", flagNames.appVersion)
		}
	}

	if flags.Changed(flagNames.valuesFile) && !flags.Changed(flagNames.release) {
		return fmt.Errorf("--%s requires a release be specified with --%s", flagNames.valuesFile, flagNames.release)
	}

	if flags.Changed(flagNames.argocd) {
		if flags.Changed(flagNames.chartDir) || flags.Changed(flagNames.chartVersion) || flags.Changed(flagNames.appVersion) || flags.Changed(flagNames.valuesFile) {
			return fmt.Errorf("--%s cannot be used with --%s, --%s, --%s, or --%s", flagNames.argocd, flagNames.chartDir, flagNames.chartVersion, flagNames.appVersion, flagNames.valuesFile)
		}
	}

	if flags.Changed(flagNames.stdout) && flags.Changed(flagNames.outputDir) {
		// can't render to both stdout and directory
		return fmt.Errorf("--%s cannot be used with --%s", flagNames.stdout, flagNames.outputDir)
	}

	return nil
}

// Adjust logging verbosity based on CLI options
func adjustLoggingVerbosity(verbosity int) {
	if verbosity > 1 {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else if verbosity > 0 {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

// Expand relative path to absolute.
// This is necessary for many arguments because Helmfile assumes paths
// are relative to helmfile.yaml and we want them to be relative to CWD.
func expandAndVerifyExists(filePath string, description string) (string, error) {
	expanded, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(expanded); os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist: %s", description, expanded)
	} else if err != nil {
		return "", fmt.Errorf("error reading %s %s: %v", description, expanded, err)
	}

	return expanded, nil
}

// Look up an nev var, first in this CLI instance's overrides, and then in the process env vars
func (cli * CLI) lookupEnvVar(name string) (value string, defined bool) {
	value, defined = cli.envVarOverrides[name]
	if !defined {
		value, defined = os.LookupEnv(name)
	}
	return
}
