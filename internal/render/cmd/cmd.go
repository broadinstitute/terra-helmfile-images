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

/* Default strings for options */
const OptionUnset = ""
const OptionAll = "all"

/* Struct encapsulating options for a render */
type Options struct {
	App          string
	Env          string
	ChartVersion string
	AppVersion   string
	ChartPath    string
	OutputDir    string
	ArgocdMode   bool
	Stdout       bool
	Verbose      int
	ConfigRepoPath string
}

/* Struct encapsulating configuration/state for a render command */
type RenderCommand struct {
	options *Options
	cobraCommand *cobra.Command
}

/*
This package handles CLI option parsing the render utility. It uses Cobra in accordance with the
documentation here: https://github.com/spf13/cobra/blob/master/user_guide.md
*/

/* Main method/entrypoint for the render program */
func Execute() {
	renderCommand := NewRenderCommand()
	if err := renderCommand.cobraCommand.Execute(); err != nil {
		log.Err(err)
		os.Exit(1)
	}

	render, err := render.NewRender(renderCommand.options)
	if err != nil {
		log.Err(err)
		os.Exit(1)
	}

	render.HelmUpdate()
	render.Render()
}

/* Initialize a new RenderCommand */
func NewRenderCommand() *RenderCommand {
	options := new(Options)
	cobraCommand := &cobra.Command{
		Use:   "render",
		Short: "Renders Terra Kubernetes manifests",
		Long: `Renders Terra Kubernetes manifests

	Examples:

		# RenderCommand all manifests for all Terra services in all environments
		$0

		# RenderCommand manifests for all Terra services in the dev environment
		$0 -e dev

		# RenderCommand manifests for the cromwell service in the alpha environment
		$0 -e alpha -a cromwell

		# RenderCommand manifests for the cromwell service in the alpha environment,
		# overriding app and chart version
		$0 -e alpha -a cromwell --chart-version="~> 0.8" --app-version="53-9b11416"

		# RenderCommand manifests from a local copy of a chart
		$0 -e alpha -a cromwell --chart-dir=../terra-helm/charts

		# RenderCommand all manifests to a directory called my-manifests
		$0 --output-dir=/tmp/my-manifests

		# RenderCommand ArgoCD manifests for all Terra services in all environments
		$0 --argocd

		# RenderCommand ArgoCD manifests for the Cromwell service in the alpha environment
		$0 -e alpha -a cromwell --argocd
	`,
		RunE: func(cobraCommand *cobra.Command, args []string) error {
			// Note: All we do in this callback is verify some CLI options; most logic
			// is in the Execute function
			return verifyCLIOptions(options)
		},
	}

	// Identify current working directory, which is used in some option defaults
	cwd, err := os.Getwd()
	if err != nil {
		log.Error().Msgf("Could not identify current working directory! %v", err)
		os.Exit(1)
	}

	cobraCommand.Flags().StringVarP(&options.App, "app", "a", OptionAll, "RenderCommand manifests for a specific Terra application only")
	cobraCommand.Flags().StringVarP(&options.Env, "env", "e", OptionAll, "RenderCommand manifests for a specific Terra environment only")
	cobraCommand.Flags().StringVar(&options.ChartVersion, "chart-version", OptionUnset, "Override chart version")
	cobraCommand.Flags().StringVar(&options.ChartPath,"chart-path", OptionUnset, "RenderCommand from local chart instead of official release")
	cobraCommand.Flags().StringVar(&options.AppVersion, "app-version", OptionUnset, "Override application version")
	cobraCommand.Flags().StringVarP(&options.OutputDir, "output-dir", "d", path.Join(cwd, "output"), "RenderCommand manifests to custom output directory")
	cobraCommand.Flags().BoolVar(&options.Stdout, "stdout", false, "RenderCommand manifests to stdout instead of output directory")
	cobraCommand.Flags().BoolVar(&options.ArgocdMode, "argocd", false, "RenderCommand ArgoCD manifests instead of application manifests")
	cobraCommand.Flags().CountVarP(&options.Verbose, "verbose", "v", "Verbose logging. Can be specified multiple times")

	// We require configRepoPath to be set via environemnt variable
	configRepoPath, defined := os.LookupEnv(EnvVarConfigRepoPath)
	if !defined {
		log.Error().Msgf("Please specify path to terra-helmfile clone via the environment variable %s", EnvVarConfigRepoPath)
		os.Exit(1)
	}
	options.ConfigRepoPath = configRepoPath

	renderCommand := new(RenderCommand)
	renderCommand.cobraCommand = cobraCommand
	renderCommand.options = options

	return renderCommand
}

func init() {
	// Initialize logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Level(zerolog.InfoLevel)
}

/* Additional verification for CLI options */
func verifyCLIOptions(opts *Options) error {
	if opts.Env == OptionAll && opts.App != OptionAll {
		/* Not all environments include all apps, so require users to specify -e with -a */
		return fmt.Errorf("an environment must be specified with -e when an app is specified with -a")
	}

	if opts.ChartPath != OptionUnset {
		if opts.ChartVersion != OptionUnset {
			return fmt.Errorf("only one of --chart-path or --chart-version may be specified")
		}

		if opts.App == OptionAll {
			return fmt.Errorf("--chart-path requires an app be specified with -a")
		}

		if _, err := os.Stat(opts.ChartPath); os.IsNotExist(err) {
			return fmt.Errorf("chart path does not exist: %s", opts.ChartPath)
		}
	}

	if opts.ChartVersion != OptionUnset {
		if opts.App == OptionAll {
			return fmt.Errorf("--chart-version requires an app be specified with -a")
		}
	}

	if opts.AppVersion != OptionUnset {
		if opts.App == OptionUnset {
			return fmt.Errorf("--app-version requires an app be specified with -a")
		}
	}

	return nil
}
