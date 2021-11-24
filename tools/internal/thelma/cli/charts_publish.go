package cli

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/source"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/cli/builders"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/cli/printing"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/versions"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const chartsPublishHelpMessage = `Publishes Helm charts for Terra services`
const chartsPublishDefaultBucketName = "terra-helm"

type chartsPublishOptions struct {
	chartDir   string
	bucketName string
	dryRun     bool
	charts     []string
}

var chartsPublishFlagNames = struct {
	chartDir   string
	bucketName string
	dryRun     string
}{
	chartDir:   "chart-dir",
	bucketName: "bucket",
	dryRun:     "dry-run",
}

type chartsPublishCLI struct {
	ctx          *ThelmaContext
	cobraCommand *cobra.Command
	options      *chartsPublishOptions
}

func newChartsPublishCLI(ctx *ThelmaContext) *chartsPublishCLI {
	options := chartsPublishOptions{}

	cobraCommand := &cobra.Command{
		Use:   "publish [options] [CHART1] [CHART2] ...",
		Short: "Publishes Helm charts",
		Long:  chartsPublishHelpMessage,
	}

	cobraCommand.Flags().StringVar(&options.chartDir, chartsPublishFlagNames.chartDir, "path/to/charts", "Publish charts from custom directory")
	cobraCommand.Flags().StringVar(&options.bucketName, chartsPublishFlagNames.bucketName, chartsPublishDefaultBucketName, "Publish charts to custom GCS bucket")
	cobraCommand.Flags().BoolVarP(&options.dryRun, chartsPublishFlagNames.dryRun, "n", false, "Dry run (don't actually update Helm repo)")

	printer := printing.NewPrinter()
	printer.AddFlags(cobraCommand)

	cobraCommand.PreRunE = func(cmd *cobra.Command, args []string) error {
		options.charts = args

		if cmd.Flags().Changed(chartsPublishFlagNames.chartDir) {
			expanded, err := expandAndVerifyExists(options.chartDir, "chart directory")
			if err != nil {
				return err
			}
			options.chartDir = expanded
		} else {
			options.chartDir = ctx.app.Paths.DefaultChartSrcDir()
		}

		if err := printer.VerifyFlags(); err != nil {
			return err
		}

		return nil
	}

	cobraCommand.RunE = func(cmd *cobra.Command, args []string) error {
		releasedVersions, err := publishCharts(&options, ctx.app)
		if err != nil {
			return err
		}
		return printer.PrintOutput(releasedVersions, cmd.OutOrStdout())
	}

	return &chartsPublishCLI{
		ctx:          ctx,
		cobraCommand: cobraCommand,
		options:      &options,
	}
}

func publishCharts(options *chartsPublishOptions, app *app.ThelmaApp) (map[string]string, error) {
	if len(options.charts) == 0 {
		log.Warn().Msgf("No charts specified; exiting")
		return make(map[string]string), nil
	}

	pb, err := builders.Publisher(app, options.bucketName, options.dryRun)
	if err != nil {
		return nil, err
	}
	defer pb.CloseWarn()

	_versions := versions.NewVersions(app.Config.Home(), app.ShellRunner)

	chartsDir, err := source.NewChartsDir(options.chartDir, pb.Publisher(), _versions, app.ShellRunner)
	if err != nil {
		return nil, err
	}

	return chartsDir.Release(options.charts)
}
