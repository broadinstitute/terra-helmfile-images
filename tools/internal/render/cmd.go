package render

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"os"
	"path"
	"path/filepath"
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

/* Default value for string CLI options */
const optionUnset = ""

/* Main method/entrypoint for the render tool. */
func Execute() {
	cobraCommand := newCobraCommand()

	if err := cobraCommand.Execute(); err != nil {
		log.Error().Msgf("%v", err)
		os.Exit(1)
	}
}

/* Construct new Cobra command */
func newCobraCommand() *cobra.Command {
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
`,
		// Only print out usage error when user supplies -h/--help
	    SilenceUsage: true,

	    // Don't print errors, we do it ourselves using a logging library
		SilenceErrors: true,

	    // Main body of the command
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("expected no positional arguments, got %v", args)
			}

			if err := normalizePaths(options); err != nil {
				return err
			}
			if err := checkIncompatibleFlags(options); err != nil {
				return err
			}
			adjustLoggingVerbosity(options.Verbose)

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
		},
	}

	cobraCommand.Flags().StringVarP(&options.App, "app", "a", optionUnset, "Render manifests for a specific Terra application only")
	cobraCommand.Flags().StringVarP(&options.Env, "env", "e", optionUnset, "Render manifests for a specific Terra environment only")
	cobraCommand.Flags().StringVar(&options.ChartVersion, "chart-version", optionUnset, "Override chart version")
	cobraCommand.Flags().StringVar(&options.ChartDir, "chart-dir", optionUnset, "Render from local chart directory instead of official release")
	cobraCommand.Flags().StringVar(&options.AppVersion, "app-version", optionUnset, "Override application version")
	cobraCommand.Flags().StringVarP(&options.OutputDir, "output-dir", "d", optionUnset, "Render manifests to custom output directory")
	cobraCommand.Flags().BoolVar(&options.Stdout, "stdout", false, "Render manifests to stdout instead of output directory")
	cobraCommand.Flags().BoolVar(&options.ArgocdMode, "argocd", false, "Render ArgoCD manifests instead of application manifests")
	cobraCommand.Flags().CountVarP(&options.Verbose, "verbose", "v", "Verbose logging. Can be specified multiple times")

	return cobraCommand
}

func init() {
	// Initialize logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

/* Check Options for incompatible flags */
func checkIncompatibleFlags(options *Options) error {
	if isSet(options.App) && !isSet(options.Env) {
		/* Not all environments include all apps, so require users to specify -e with -a */
		return fmt.Errorf("an environment must be specified with -e when an app is specified with -a")
	}

	if isSet(options.ChartDir) {
		if isSet(options.ChartVersion) {
			return fmt.Errorf("only one of --chart-dir or --chart-version may be specified")
		}

		if !isSet(options.App) {
			return fmt.Errorf("--chart-dir requires an app be specified with -a")
		}
	}

	if isSet(options.ChartVersion) && !isSet(options.App) {
		return fmt.Errorf("--chart-version requires an app be specified with -a")
	}

	if isSet(options.AppVersion) && !isSet(options.App) {
		return fmt.Errorf("--app-version requires an app be specified with -a")
	}

	if options.ArgocdMode {
		if isSet(options.ChartDir) || isSet(options.ChartVersion) || isSet(options.AppVersion) {
			return fmt.Errorf("--argocd cannot be used with --chart-dir, --chart-version, or --app-version")
		}
	}

	return nil
}

/* Normalize and validate path arguments */
func normalizePaths(options *Options) error {
	// We require configRepoPath to be set via environment variable
	configRepoPath, defined := os.LookupEnv(ConfigRepoPathEnvVar)
	if !defined {
		return fmt.Errorf("please specify path to %s clone via the environment variable %s", ConfigRepoName, ConfigRepoPathEnvVar)
	}
	options.ConfigRepoPath = configRepoPath

	// Expand relative path arguments to absolute paths.
	// This is because Helmfile assumes paths are relative to helmfile.yaml
	// and we want them to be relative to CWD.
	var err error

	if isSet(options.ChartDir) {
		if options.ChartDir, err = filepath.Abs(options.ChartDir); err != nil {
			return err
		}
		if _, err = os.Stat(options.ChartDir); os.IsNotExist(err) {
			return fmt.Errorf("chart directory does not exist: %s", options.ChartDir)
		}
	}

	if options.Stdout {
		if isSet(options.OutputDir) {
			return fmt.Errorf("--stdout cannot be used with -d/--output-dir")
		} else {
			return nil
		}
	}

	// --stdout is not set, so set default output directory $CONFIG_REPO_PATH/output
	// if a custom output dir was not specified on the command-line
	if !isSet(options.OutputDir) {
		options.OutputDir = path.Join(configRepoPath, DefaultOutputDirName)
		log.Debug().Msgf("Using default output dir %s", options.OutputDir)
	}

	if options.OutputDir, err = filepath.Abs(options.OutputDir); err != nil {
		return err
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

/* Short-hand helper function indicating whether a string option was set by the user */
func isSet(optionValue string) bool {
	return optionValue != optionUnset
}
