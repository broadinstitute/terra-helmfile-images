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

// This file handles CLI option parsing the render utility. It uses Cobra in accordance with the
// documentation here: https://github.com/spf13/cobra/blob/master/user_guide.md

// Name of the Helmfile configuration repo
const configRepoName = "terra-helmfile"

// Environment variable used to set path to config repo clone
const configRepoPathEnvVar = "TERRA_HELMFILE_PATH"

// Name of default output directory
const defaultOutputDirName = "output"

// Default value for string CLI options
const optionUnset = ""

// Execute is the main method/entrypoint for the render tool.
func Execute() {
	cobraCommand := newCobraCommand()

	if err := cobraCommand.Execute(); err != nil {
		log.Error().Msgf("%v", err)
		os.Exit(1)
	}
}

// Construct new Cobra command
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

# Render manifests, overriding chart values with a local file
$0 -e alpha -a cromwell --values-file=path/to/my-values.yaml

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

			if err := checkIncompatibleFlags(options); err != nil {
				return err
			}
			if err := normalizePaths(options); err != nil {
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

	cobraCommand.Flags().StringVarP(&options.Env, "env", "e", optionUnset, "Render manifests for a specific Terra environment only")
	cobraCommand.Flags().StringVarP(&options.Cluster, "cluster", "c", optionUnset, "Render manifests for a specific Terra cluster only")
	cobraCommand.Flags().StringVarP(&options.Release, "release", "r", optionUnset, "Render manifests for a specific release only")
	cobraCommand.Flags().StringVarP(&options.Release, "app", "a", optionUnset, "Render manifests for a specific app only. (Alias for -r/--release)")
	cobraCommand.Flags().StringVar(&options.ChartVersion, "chart-version", optionUnset, "Override chart version")
	cobraCommand.Flags().StringVar(&options.ChartDir, "chart-dir", optionUnset, "Render from local chart directory instead of official release")
	cobraCommand.Flags().StringVar(&options.AppVersion, "app-version", optionUnset, "Override application version")
	cobraCommand.Flags().StringSliceVar(&options.ValuesFiles, "values-file", []string{}, "Path to chart values file. Can be specified multiple times with ascending precedence (last wins)")
	cobraCommand.Flags().BoolVar(&options.ArgocdMode, "argocd", false, "Render ArgoCD manifests instead of application manifests")
	cobraCommand.Flags().StringVarP(&options.OutputDir, "output-dir", "d", optionUnset, "Render manifests to custom output directory")
	cobraCommand.Flags().BoolVar(&options.Stdout, "stdout", false, "Render manifests to stdout instead of output directory")
	cobraCommand.Flags().CountVarP(&options.Verbose, "verbose", "v", "Verbose logging. Can be specified multiple times")

	return cobraCommand
}

func init() {
	// Initialize logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

// Check Options for incompatible flags
func checkIncompatibleFlags(options *Options) error {
	if isSet(options.Release) && !(isSet(options.Env) || isSet(options.Cluster)) {
		// Not all targets include all charts, so require users to specify target env or cluster with -a
		return fmt.Errorf("an environment (-e) or cluster (-c) must be specified when a release is specified with -r")
	}

	if isSet(options.ChartDir) {
		if isSet(options.ChartVersion) {
			return fmt.Errorf("only one of --chart-dir or --chart-version may be specified")
		}

		if !isSet(options.Release) {
			return fmt.Errorf("--chart-dir requires a release be specified with -r")
		}
	}

	if isSet(options.ChartVersion) && !isSet(options.Release) {
		return fmt.Errorf("--chart-version requires a release be specified with -r")
	}

	if isSet(options.AppVersion) {
		if !isSet(options.Release) {
			return fmt.Errorf("--app-version requires a release be specified with -r")
		}
		if isSet(options.Cluster) {
			return fmt.Errorf("--app-version cannot be used for cluster releases")
		}
	}

	if len(options.ValuesFiles) > 0 && !isSet(options.Release) {
		return fmt.Errorf("--values-file requires a release be specified with -r")
	}

	if options.ArgocdMode {
		if isSet(options.ChartDir) || isSet(options.ChartVersion) || isSet(options.AppVersion) || len(options.ValuesFiles) != 0 {
			return fmt.Errorf("--argocd cannot be used with --chart-dir, --chart-version, --app-version, or --values-file")
		}
	}

	return nil
}

// Normalize and validate path arguments.
func normalizePaths(options *Options) error {
	if err := normalizeConfigRepoPath(options); err != nil {
		return err
	}

	if isSet(options.ChartDir) {
		if err := normalizeChartDir(options); err != nil {
			return err
		}
	}

	if len(options.ValuesFiles) > 0 {
		if err := normalizeValuesFiles(options); err != nil {
			return err
		}
	}

	return normalizeOutputDir(options)
}

// Adjust logging verbosity based on CLI options
func adjustLoggingVerbosity(verbosity int) {
	if verbosity > 1 {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else if verbosity > 0 {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

// Short-hand helper function indicating whether a string option was set by the user
func isSet(optionValue string) bool {
	return optionValue != optionUnset
}

// Validate config repo path and add to Options
func normalizeConfigRepoPath(options *Options) error {
	// We require configRepoPath to be set via environment variable
	configRepoPath, defined := os.LookupEnv(configRepoPathEnvVar)
	if !defined {
		return fmt.Errorf("please specify path to %s clone via the environment variable %s", configRepoName, configRepoPathEnvVar)
	}
	options.ConfigRepoPath = configRepoPath
	return nil
}

// Validate chart dir, expand it, and add to Options
func normalizeChartDir(options *Options) error {
	expanded, err := expandAndVerifyExists(options.ChartDir, "chart directory")
	if err != nil {
		return err
	}
	options.ChartDir = *expanded
	return nil
}

// For every --values-file arguments, expand to full path and verify it exists
// Then update Options to use the normalized paths
func normalizeValuesFiles(options *Options) error {
	var expandedValuesFiles []string

	for _, valuesFile := range options.ValuesFiles {
		expanded, err := expandAndVerifyExists(valuesFile, "values file")
		if err != nil {
			return err
		}
		expandedValuesFiles = append(expandedValuesFiles, *expanded)
	}
	options.ValuesFiles = expandedValuesFiles

	return nil
}

// If an output dir was given, validate it. Else update Options with the default
// output directory, $CONFIG_REPO_PATH/output
//
// Note: This MUST be called after normalizeConfigRepoPath()
func normalizeOutputDir(options *Options) error {
	if options.Stdout {
		if isSet(options.OutputDir) {
			return fmt.Errorf("--stdout cannot be used with -d/--output-dir")
		}

		// We're rendering to stdout, so no need to set up default output dir
		return nil
	}

	// No output directory was given on the command-line, so set to default output
	// directory $CONFIG_REPO_PATH/output
	if !isSet(options.OutputDir) {
		options.OutputDir = path.Join(options.ConfigRepoPath, defaultOutputDirName)
		log.Debug().Msgf("Using default output dir %s", options.OutputDir)
	}

	// Normalize path to output dir, whether it's the default or user-supplied
	dir, err := filepath.Abs(options.OutputDir)
	if err != nil {
		return err
	}
	options.OutputDir = dir

	return nil
}

// Expand relative path to absolute.
// This is necessary for many arguments because Helmfile assumes paths
// are relative to helmfile.yaml and we want them to be relative to CWD.
func expandAndVerifyExists(filePath string, description string) (*string, error) {
	expanded, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(expanded); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist: %s", description, expanded)
	} else if err != nil {
		return nil, fmt.Errorf("error reading %s %s: %v", description, expanded, err)
	}

	return &expanded, nil
}
