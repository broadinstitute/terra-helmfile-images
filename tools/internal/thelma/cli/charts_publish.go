package cli

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/source"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/cli/builders"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/versions"
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

	cobraCommand.PreRunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("at least one chart must be specified")
		}
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

		return nil
	}

	cobraCommand.RunE = func(cmd *cobra.Command, args []string) error {
		return publishCharts(&options, ctx.app)
	}

	return &chartsPublishCLI{
		ctx:          ctx,
		cobraCommand: cobraCommand,
		options:      &options,
	}
}

func publishCharts(options *chartsPublishOptions, app *app.ThelmaApp) error {
	pb, err := builders.Publisher(app, options.bucketName, options.dryRun)
	if err != nil {
		return err
	}
	defer pb.CloseWarn()

	_versions := versions.NewVersions(app.Config.Home(), app.ShellRunner)

	chartsDir, err := source.NewChartsDir(options.chartDir, pb.Publisher(), _versions, app.ShellRunner)
	if err != nil {
		return err
	}

	return chartsDir.Release(options.charts)
}
