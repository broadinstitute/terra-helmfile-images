package cli

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/helmfile"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/resolver"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"path"
	"path/filepath"
)

// This file handles option parsing for the `render` subcommand.

const renderHelpMessage = `Renders Terra Kubernetes manifests

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

// defaultRenderOutputDir name of default output directory
const defaultRenderOutputDir = "output"

// defaultRenderChartSourceDir name of default chart source directory
const defaultRenderChartSourceDir = "charts"

// renderCLI contains state and configuration for executing a render from the command-line
type renderCLI struct {
	ctx           *ThelmaContext
	cobraCommand  *cobra.Command
	helmfileArgs  *helmfile.Args
	renderOptions *render.Options
	flagVals      *renderFlagValues
}

// Names of all the CLI flags are kept in a struct so they can be easily referenced in error messages
var renderFlagNames = struct {
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
	stdout          string
	parallelWorkers string
	mode            string
}{
	env:             "env",
	cluster:         "cluster",
	app:             "app",
	release:         "release",
	chartDir:        "chart-dir",
	chartVersion:    "chart-version",
	appVersion:      "app-version",
	valuesFile:      "values-file",
	argocd:          "argocd",
	outputDir:       "output-dir",
	stdout:          "stdout",
	parallelWorkers: "parallel-workers",
	mode:            "mode",
}

// renderFlagValues is a struct for capturing flag values that are parsed by Cobra.
type renderFlagValues struct {
	env             string
	cluster         string
	app             string
	release         string
	chartDir        string
	chartVersion    string
	appVersion      string
	valuesFile      []string
	argocd          bool
	outputDir       string
	stdout          bool
	parallelWorkers int
	mode            string
}

// newRenderCLI constructs a new renderCLI
func newRenderCLI(ctx *ThelmaContext) *renderCLI {
	flagVals := &renderFlagValues{}
	helmfileArgs := &helmfile.Args{}
	renderOptions := &render.Options{}

	cobraCommand := &cobra.Command{
		Use:           "render [options]",
		Short:         "Renders Terra Kubernetes manifests",
		Long:          renderHelpMessage,
		SilenceUsage:  true, // Only print out usage error when user supplies -h/--help
		SilenceErrors: true, // Don't print errors, we do it ourselves using a logging library
	}

	// Add CLI flag handlers to cobra command
	cobraCommand.Flags().StringVarP(&flagVals.env, renderFlagNames.env, "e", "ENV", "Render manifests for a specific Terra environment only")
	cobraCommand.Flags().StringVarP(&flagVals.cluster, renderFlagNames.cluster, "c", "CLUSTER", "Render manifests for a specific Terra cluster only")
	cobraCommand.Flags().StringVarP(&flagVals.release, renderFlagNames.release, "r", "RELEASE", "Render manifests for a specific release only")
	cobraCommand.Flags().StringVarP(&flagVals.app, renderFlagNames.app, "a", "APP", "Render manifests for a specific app only. (Alias for -r/--release)")
	cobraCommand.Flags().StringVar(&flagVals.chartVersion, renderFlagNames.chartVersion, "VERSION", "Override chart version")
	cobraCommand.Flags().StringVar(&flagVals.chartDir, renderFlagNames.chartDir, "path/to/charts", "Render from local chart directory instead of official release")
	cobraCommand.Flags().StringVar(&flagVals.appVersion, renderFlagNames.appVersion, "VERSION", "Override application version")
	cobraCommand.Flags().StringSliceVar(&flagVals.valuesFile, renderFlagNames.valuesFile, []string{}, "path to chart values file. Can be specified multiple times with ascending precedence (last wins)")
	cobraCommand.Flags().BoolVar(&flagVals.argocd, renderFlagNames.argocd, false, "Render ArgoCD manifests instead of application manifests")
	cobraCommand.Flags().StringVarP(&flagVals.outputDir, renderFlagNames.outputDir, "d", "path/to/output/dir", "Render manifests to custom output directory")
	cobraCommand.Flags().BoolVar(&flagVals.stdout, renderFlagNames.stdout, false, "Render manifests to stdout instead of output directory")
	cobraCommand.Flags().IntVar(&flagVals.parallelWorkers, renderFlagNames.parallelWorkers, 1, "Number of parallel workers to launch when rendering")
	cobraCommand.Flags().StringVar(&flagVals.mode, renderFlagNames.mode, "development", `Either "development" (render from chart source directory) or "deploy" (render using released chart versions). Defaults to "development"`)

	cli := &renderCLI{
		cobraCommand:  cobraCommand,
		renderOptions: renderOptions,
		helmfileArgs:  helmfileArgs,
		flagVals:      flagVals,
		ctx:           ctx,
	}

	cobraCommand.PreRunE = func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("expected no positional arguments, got %v", args)
		}
		if err := cli.handleFlagAliases(); err != nil {
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

		return nil
	}

	cobraCommand.RunE = func(cmd *cobra.Command, args []string) error {
		return render.DoRender(ctx.app, renderOptions, helmfileArgs)

	}

	return cli
}

// fillRenderOptions populates an empty render.Options struct in accordance with user-supplied CLI options
func (cli *renderCLI) fillRenderOptions() error {
	flags := cli.cobraCommand.Flags()
	flagVals := cli.flagVals
	renderOptions := cli.renderOptions

	// env
	if flags.Changed(renderFlagNames.env) {
		renderOptions.Env = &flagVals.env
	}

	// cluster
	if flags.Changed(renderFlagNames.cluster) {
		renderOptions.Cluster = &flagVals.cluster
	}

	// release name
	if flags.Changed(renderFlagNames.release) {
		renderOptions.Release = &flagVals.release
	}

	// output dir
	if flags.Changed(renderFlagNames.outputDir) {
		dir, err := filepath.Abs(flagVals.outputDir)
		if err != nil {
			return err
		}
		renderOptions.OutputDir = dir
	} else {
		renderOptions.OutputDir = path.Join(cli.ctx.app.Config.Home(), defaultRenderOutputDir)
		log.Debug().Msgf("Using default output dir %s", renderOptions.OutputDir)
	}

	// stdout
	renderOptions.Stdout = flagVals.stdout

	// parallelWorkers
	renderOptions.ParallelWorkers = flagVals.parallelWorkers

	// chart dir
	if flags.Changed(renderFlagNames.chartDir) {
		chartSourceDir, err := expandAndVerifyExists(flagVals.chartDir, "chart dir")
		if err != nil {
			return err
		}
		renderOptions.ChartSourceDir = chartSourceDir
	} else {
		renderOptions.ChartSourceDir = path.Join(cli.ctx.app.Config.Home(), defaultRenderChartSourceDir)
		log.Debug().Msgf("Using default chart source dir %s", renderOptions.ChartSourceDir)
	}

	// resolve mode
	switch flagVals.mode {
	case "development":
		renderOptions.ResolverMode = resolver.Development
	case "deploy":
		renderOptions.ResolverMode = resolver.Deploy
	default:
		return fmt.Errorf(`invalid value for --%s (must be "development" or "deploy"): %v`, renderFlagNames.mode, flagVals.mode)
	}

	return nil
}

// fillHelmfileArgs populates an empty helfile.Args struct in accordance with user-supplied CLI options
func (cli *renderCLI) fillHelmfileArgs() error {
	flags := cli.cobraCommand.Flags()
	flagVals := cli.flagVals
	helmfileArgs := cli.helmfileArgs

	// chart version
	if flags.Changed(renderFlagNames.chartVersion) {
		helmfileArgs.ChartVersion = &flagVals.chartVersion
	}

	// app version
	if flags.Changed(renderFlagNames.appVersion) {
		helmfileArgs.AppVersion = &flagVals.appVersion
	}

	// values file
	if flags.Changed(renderFlagNames.valuesFile) {
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
func (cli *renderCLI) handleFlagAliases() error {
	flags := cli.cobraCommand.Flags()

	// --app is a legacy alias for --release, so copy the user-supplied value over
	if flags.Changed(renderFlagNames.app) {
		if flags.Changed(renderFlagNames.release) {
			return fmt.Errorf("-a is a legacy alias for -r, please specify one or the other but not both")
		}
		_app := flags.Lookup(renderFlagNames.app).Value.String()
		err := flags.Set(renderFlagNames.release, _app)
		if err != nil {
			return fmt.Errorf("error setting --%s to --%s argument %q: %v", renderFlagNames.release, renderFlagNames.app, _app, err)
		}
	}

	return nil
}

func (cli *renderCLI) checkIncompatibleFlags() error {
	flags := cli.cobraCommand.Flags()

	if flags.Changed(renderFlagNames.env) && flags.Changed(renderFlagNames.cluster) {
		return fmt.Errorf("only one of --%s or --%s may be specified", renderFlagNames.env, renderFlagNames.cluster)
	}

	if flags.Changed(renderFlagNames.chartDir) {
		if flags.Changed(renderFlagNames.chartVersion) {
			// Chart dir points at a local chart copy, chart version specifies which version to use, we can only
			// use one or the other
			return fmt.Errorf("only one of --%s or --%s may be specified", renderFlagNames.chartDir, renderFlagNames.chartVersion)
		}

		if !flags.Changed(renderFlagNames.release) {
			return fmt.Errorf("--%s requires a release be specified with --%s", renderFlagNames.chartDir, renderFlagNames.release)
		}
	}

	if flags.Changed(renderFlagNames.chartVersion) && !flags.Changed(renderFlagNames.release) {
		return fmt.Errorf("--%s requires a release be specified with --%s", renderFlagNames.chartVersion, renderFlagNames.release)
	}

	if flags.Changed(renderFlagNames.appVersion) {
		if !flags.Changed(renderFlagNames.release) {
			return fmt.Errorf("--%s requires a release be specified with --%s", renderFlagNames.appVersion, renderFlagNames.release)
		}
		if flags.Changed(renderFlagNames.cluster) {
			return fmt.Errorf("--%s cannot be used for cluster releases", renderFlagNames.appVersion)
		}
	}

	if flags.Changed(renderFlagNames.valuesFile) && !flags.Changed(renderFlagNames.release) {
		return fmt.Errorf("--%s requires a release be specified with --%s", renderFlagNames.valuesFile, renderFlagNames.release)
	}

	if flags.Changed(renderFlagNames.argocd) {
		if flags.Changed(renderFlagNames.chartDir) || flags.Changed(renderFlagNames.chartVersion) || flags.Changed(renderFlagNames.appVersion) || flags.Changed(renderFlagNames.valuesFile) {
			return fmt.Errorf("--%s cannot be used with --%s, --%s, --%s, or --%s", renderFlagNames.argocd, renderFlagNames.chartDir, renderFlagNames.chartVersion, renderFlagNames.appVersion, renderFlagNames.valuesFile)
		}
	}

	if flags.Changed(renderFlagNames.stdout) && flags.Changed(renderFlagNames.outputDir) {
		// can't render to both stdout and directory
		return fmt.Errorf("--%s cannot be used with --%s", renderFlagNames.stdout, renderFlagNames.outputDir)
	}

	if flags.Changed(renderFlagNames.parallelWorkers) && flags.Changed(renderFlagNames.stdout) {
		// --parallel-workers renders manifests in parallel. For now we only support it for directory renders
		return fmt.Errorf("--%s cannot be used with --%s", renderFlagNames.parallelWorkers, renderFlagNames.stdout)
	}
	return nil
}
