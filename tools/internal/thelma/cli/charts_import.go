package cli

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/mirror"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/cli/builders"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/cli/printing"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/cli/views"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"path"
)

const chartsImportHelpMessage = `Imports charts from public Helm repositories into the terra-helm-third-party-repo`
const chartsImportDefaultBucketName = "terra-helm-thirdparty"
const chartsImportDefaultConfigFile = "third-party-charts.yaml"

type chartsImportOptions struct {
	configFile string
	bucketName string
	dryRun     bool
}

var chartsImportFlagNames = struct {
	configFile string
	bucketName string
	dryRun     string
}{
	configFile: "config-from",
	bucketName: "bucket",
	dryRun:     "dry-run",
}

type chartsImportCLI struct {
	ctx          *ThelmaContext
	cobraCommand *cobra.Command
	options      *chartsImportOptions
}

func newChartsImportCLI(ctx *ThelmaContext) *chartsImportCLI {
	options := chartsImportOptions{}

	cobraCommand := &cobra.Command{
		Use:   "import [options]",
		Short: "Imports third-party Helm charts into the terra-helm-thirdparty repo",
		Long:  chartsImportHelpMessage,
	}

	cobraCommand.Flags().StringVar(&options.configFile, chartsImportFlagNames.configFile, path.Join("$THELMA_HOME", "etc", chartsImportDefaultConfigFile), "Path to import config file")
	cobraCommand.Flags().StringVar(&options.bucketName, chartsImportFlagNames.bucketName, chartsImportDefaultBucketName, "Publish charts to custom GCS bucket")
	cobraCommand.Flags().BoolVarP(&options.dryRun, chartsImportFlagNames.dryRun, "n", false, "Dry run (don't actually update Helm repo)")

	printer := printing.NewPrinter()
	printer.AddFlags(cobraCommand)

	cobraCommand.PreRunE = func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("expected no positional arguments, got %v", args)
		}
		if cmd.Flags().Changed(chartsImportFlagNames.configFile) {
			expanded, err := expandAndVerifyExists(options.configFile, "configFile")
			if err != nil {
				return err
			}
			options.configFile = expanded
		} else {
			options.configFile = path.Join(ctx.app.Paths.MiscConfDir(), chartsImportDefaultConfigFile)
		}

		if err := printer.VerifyFlags(); err != nil {
			return err
		}

		return nil
	}

	cobraCommand.RunE = func(cmd *cobra.Command, args []string) error {
		imported, err := importCharts(&options, ctx.app)
		if err != nil {
			return err
		}

		if options.dryRun {
			log.Info().Msgf("This is a dry run; would have imported %d charts to %s", len(imported), options.bucketName)
		} else {
			log.Info().Msgf("Imported %d charts to %s", len(imported), options.bucketName)
		}

		return printer.PrintOutput(imported, cmd.OutOrStdout())
	}

	return &chartsImportCLI{
		ctx:          ctx,
		cobraCommand: cobraCommand,
		options:      &options,
	}
}

func importCharts(options *chartsImportOptions, app *app.ThelmaApp) ([]views.ChartRelease, error) {
	pb, err := builders.Publisher(app, options.bucketName, options.dryRun)
	if err != nil {
		return nil, err
	}
	defer pb.CloseWarn()

	_mirror, err := mirror.NewMirror(pb.Publisher(), app.ShellRunner, options.configFile)
	if err != nil {
		return nil, err
	}

	chartDefns, err := _mirror.ImportToMirror()
	if err != nil {
		return nil, err
	}

	// convert result to view
	var result []views.ChartRelease
	for _, chartDefn := range chartDefns {
		result = append(result, views.ChartRelease{
			Name:    chartDefn.ChartName(),
			Version: chartDefn.Version,
			Repo:    chartDefn.RepoName(),
		})
	}
	views.SortChartReleases(result)
	return result, nil
}
