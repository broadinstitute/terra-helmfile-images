package cmd

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"os"
	"path"
	"terra-helmfile-tools/internal/render"
)

/* Environment variable used to set path to terra-helmfile clone */
const EnvVarConfigRepoPath = "TERRA_HELMFILE_PATH"

/* File to check for when verifying terra-helmfile clone actually exists */
const ConfigRepoTestFile = "helmfile.yaml"

var (
	options = new(render.Options)
	cobraCommand = &cobra.Command{
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
	`,
	RunE: func(cobraCommand *cobra.Command, args []string) error {
		// Note: All we do in this callback is verify some CLI options; most logic
		// is in the Execute function
		if err := setConfigRepoPath(options); err != nil {
			return err
		}
		return verifyCLIOptions(options)
	},
	}
)

/*
This package handles CLI option parsing the render utility. It uses Cobra in accordance with the
documentation here: https://github.com/spf13/cobra/blob/master/user_guide.md
*/

/* Main method/entrypoint for the render tool */
func Execute() {
	if err := cobraCommand.Execute(); err != nil {
		log.Err(err)
		os.Exit(1)
	}

	adjustLoggingVerbosity(options.Verbose)

	render, err := render.NewRender(options)
	if err != nil {
		log.Err(err)
		os.Exit(1)
	}

	render.CleanOutputDirectory()
	render.HelmUpdate()
	render.RenderAll()
}

func init() {
	// Initialize logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Identify current working directory, which is used in some option defaults
	cwd, err := os.Getwd()
	if err != nil {
		log.Error().Msgf("Could not identify current working directory! %v", err)
		os.Exit(1)
	}

	// Initialize Cobra CLI flags
	cobraCommand.Flags().StringVarP(&options.App, "app", "a", render.OptionAll, "Render manifests for a specific Terra application only")
	cobraCommand.Flags().StringVarP(&options.Env, "env", "e", render.OptionAll, "Render manifests for a specific Terra environment only")
	cobraCommand.Flags().StringVar(&options.ChartVersion, "chart-version", render.OptionUnset, "Override chart version")
	cobraCommand.Flags().StringVar(&options.ChartDir,"chart-dir", render.OptionUnset, "Render from local chart directory instead of official release")
	cobraCommand.Flags().StringVar(&options.AppVersion, "app-version", render.OptionUnset, "Override application version")
	cobraCommand.Flags().StringVarP(&options.OutputDir, "output-dir", "d", path.Join(cwd, "output"), "Render manifests to custom output directory")
	cobraCommand.Flags().BoolVar(&options.Stdout, "stdout", false, "Render manifests to stdout instead of output directory")
	cobraCommand.Flags().BoolVar(&options.ArgocdMode, "argocd", false, "Render ArgoCD manifests instead of application manifests")
	cobraCommand.Flags().CountVarP(&options.Verbose, "verbose", "v", "Verbose logging. Can be specified multiple times")
}

/* Additional verification for CLI options */
func verifyCLIOptions(options *render.Options) error {
	if options.Env == render.OptionAll && options.App != render.OptionAll {
		/* Not all environments include all apps, so require users to specify -e with -a */
		return fmt.Errorf("an environment must be specified with -e when an app is specified with -a")
	}

	if options.ChartDir != render.OptionUnset {
		if options.ChartVersion != render.OptionUnset {
			return fmt.Errorf("only one of --chart-path or --chart-version may be specified")
		}

		if options.App == render.OptionAll {
			return fmt.Errorf("--chart-path requires an app be specified with -a")
		}

		if _, err := os.Stat(options.ChartDir); os.IsNotExist(err) {
			return fmt.Errorf("chart directory does not exist: %s", options.ChartDir)
		}
	}

	if options.ChartVersion != render.OptionUnset {
		if options.App == render.OptionAll {
			return fmt.Errorf("--chart-version requires an app be specified with -a")
		}
	}

	if options.AppVersion != render.OptionUnset {
		if options.App == render.OptionUnset {
			return fmt.Errorf("--app-version requires an app be specified with -a")
		}
	}

	return nil
}

/* Populate options.ConfigRepoPath from environment variable */
func setConfigRepoPath(options *render.Options) error {
	// We require configRepoPath to be set via environment variable
	configRepoPath, defined := os.LookupEnv(EnvVarConfigRepoPath)
	if !defined {
		return fmt.Errorf("Please specify path to terra-helmfile clone via the environment variable %s", EnvVarConfigRepoPath)
	}

	// Verify configRepoPath exists and includes an expected file (eg. helmfile.yaml)
	testFile := path.Join(configRepoPath, ConfigRepoTestFile)
	if _, err := os.Stat(testFile); err != nil {
		return fmt.Errorf("Could not find %s in %s, is it a terra-helmfile clone?", testFile, configRepoPath)
	}
	options.ConfigRepoPath = configRepoPath

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
