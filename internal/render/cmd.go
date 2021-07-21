package render

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"os"
	"path"
)

/*
This file handles CLI option parsing the render utility. It uses Cobra in accordance with the
documentation here: https://github.com/spf13/cobra/blob/master/user_guide.md
*/

/* Name of the Helmfile configuration repo */
const ConfigRepoName = "terra-helmfile"

/* Environment variable used to set path to config repo clone */
const ConfigRepoPathEnvVar = "TERRA_HELMFILE_PATH"

/* Name of default output directory */
const DefaultOutputDirName = "output"

/* Struct bundling Cobra command and Options it populates */
type CLI struct {
	options *Options
	cobraCommand *cobra.Command
}

/* Main method/entrypoint for the render tool. */
func Execute() {
	if err := ExecuteWithCallback(nil); err != nil {
		log.Fatal().Err(err)
		os.Exit(1)
	}
}

/* Execute cobra command with given arguments */
func ExecuteWithCallback(cobraCallback func(*cobra.Command)) error {
	c := newCLI()
	options, cobraCommand := c.options, c.cobraCommand

	if cobraCallback != nil {
		// Make it possible for unit tests to supply fake arguments by calling `SetArgs` on the cobra command
		cobraCallback(cobraCommand)
	}

	// Parse CLI flags
	if err := cobraCommand.Execute(); err != nil {
		return err
	}

	// Extra verification and processing, now that CLI flags have been parsed
	if err := setConfigRepoPath(options); err != nil {
		return err
	}

	if err := checkIncompatibleFlags(options); err != nil {
		return err
	}

	adjustLoggingVerbosity(options.Verbose)

	// Prepare to render manifests
	render, err := NewRender(options)
	if err != nil {
		return err
	}

	if err = render.CleanOutputDirectory(); err != nil {
		return err
	}

	if err = render.HelmUpdate(); err != nil {
		return err
	}

	if err = render.RenderAll(); err != nil {
		return err
	}

	return nil
}

func newCLI() *CLI {
	options := &Options{}
	cobraCommand := &cobra.Command{
		Use:   "render",
		Short: "Renders Terra Kubernetes manifests",
		Long: `Renders Terra Kubernetes manifests

	Examples:

		# Render all manifests for all Terra services in all environments
		$0

		# Render manifests for all Terra services in the dev environment
		$0 -e dev

		# Render manifests for the cromwell service in the alpha environment
		$0 -e alpha -a cromwell

		# Render manifests for the cromwell service in the alpha environment,
		# overriding app and chart version
		$0 -e alpha -a cromwell --chart-version="~> 0.8" --app-version="53-9b11416"

		# Render manifests from a local copy of a chart
		$0 -e alpha -a cromwell --chart-dir=../terra-helm/charts

		# Render all manifests to a directory called my-manifests
		$0 --output-dir=/tmp/my-manifests

		# Render ArgoCD manifests for all Terra services in all environments
		$0 --argocd

		# Render ArgoCD manifests for the Cromwell service in the alpha environment
		$0 -e alpha -a cromwell --argocd
	`}

	cobraCommand.Flags().StringVarP(&options.App, "app", "a", optionUnset, "Render manifests for a specific Terra application only")
	cobraCommand.Flags().StringVarP(&options.Env, "env", "e", optionUnset, "Render manifests for a specific Terra environment only")
	cobraCommand.Flags().StringVar(&options.ChartVersion, "chart-version", optionUnset, "Override chart version")
	cobraCommand.Flags().StringVar(&options.ChartDir,"chart-dir", optionUnset, "Render from local chart directory instead of official release")
	cobraCommand.Flags().StringVar(&options.AppVersion, "app-version", optionUnset, "Override application version")
	cobraCommand.Flags().StringVarP(&options.OutputDir, "output-dir", "d", optionUnset, "Render manifests to custom output directory")
	cobraCommand.Flags().BoolVar(&options.Stdout, "stdout", false, "Render manifests to stdout instead of output directory")
	cobraCommand.Flags().BoolVar(&options.ArgocdMode, "argocd", false, "Render ArgoCD manifests instead of application manifests")
	cobraCommand.Flags().CountVarP(&options.Verbose, "verbose", "v", "Verbose logging. Can be specified multiple times")

	return &CLI{options, cobraCommand}
}

func init() {
	// Initialize logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

/* Check Options for incompatible flags */
func checkIncompatibleFlags(options *Options) error {
	if IsSet(options.App) && !IsSet(options.Env) {
		/* Not all environments include all apps, so require users to specify -e with -a */
		return fmt.Errorf("an environment must be specified with -e when an app is specified with -a")
	}

	if IsSet(options.ChartDir) {
		if IsSet(options.ChartVersion) {
			return fmt.Errorf("only one of --chart-dir or --chart-version may be specified")
		}

		if !IsSet(options.App) {
			return fmt.Errorf("--chart-dir requires an app be specified with -a")
		}

		if _, err := os.Stat(options.ChartDir); os.IsNotExist(err) {
			return fmt.Errorf("chart directory does not exist: %s", options.ChartDir)
		}
	}

	if IsSet(options.ChartVersion) && !IsSet(options.App) {
		return fmt.Errorf("--chart-version requires an app be specified with -a")
	}

	if IsSet(options.AppVersion) && !IsSet(options.App) {
		return fmt.Errorf("--app-version requires an app be specified with -a")
	}

	if options.ArgocdMode {
		if IsSet(options.ChartDir) || IsSet(options.ChartVersion) || IsSet(options.AppVersion) {
			return fmt.Errorf("--argocd cannot be used with --chart-dir, --chart-version, or --app-version")
		}
	}

	return nil
}

/* Populate options.ConfigRepoPath and options.OutputDir from environment variable */
func setConfigRepoPath(options *Options) error {
	// We require configRepoPath to be set via environment variable
	configRepoPath, defined := os.LookupEnv(ConfigRepoPathEnvVar)
	if !defined {
		return fmt.Errorf("please specify path to %s clone via the environment variable %s", ConfigRepoName, ConfigRepoPathEnvVar)
	}
	options.ConfigRepoPath = configRepoPath

	// If an explicit output dir was not set, default to $CONFIG_REPO_PATH/output
	if options.OutputDir == optionUnset {
		options.OutputDir = path.Join(configRepoPath, DefaultOutputDirName)
		log.Debug().Msgf("Using default output dir %s", options.OutputDir)
	}

	return nil
}

/* Adjust logging verbosity based on CLI options */
func adjustLoggingVerbosity(verbosity int) {
	if verbosity > 1 {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else if verbosity > 0 {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}
