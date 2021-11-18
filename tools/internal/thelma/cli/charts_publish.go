package cli

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/dependency"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/publish"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo/bucket"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/source"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"strings"
)

const chartsPublishHelpMessage = `Publishes Helm charts for Terra services`
const chartsPublishDefaultBucketName = "terra-helm"

type chartsPublishOptions struct {
	chartDir string
	bucketName string
	dryRun bool
	charts []string
}
var chartsPublishFlagNames = struct {
	chartDir        string
	bucketName      string
	dryRun          string
}{
	chartDir:        "chart-dir",
	bucketName:      "bucket",
	dryRun:          "dry-run",
}

type chartsPublishCLI struct {
	ctx *ThelmaContext
	cobraCommand *cobra.Command
	options *chartsPublishOptions
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
		ctx: ctx,
		cobraCommand: cobraCommand,
		options: &options,
	}
}

func publishCharts(options *chartsPublishOptions, app *app.ThelmaApp) error {
	chartsToPublish := options.charts

	src, err := source.NewSourceDirectory(options.chartDir, app.ShellRunner)
	if err != nil {
		return err
	}

	for _, chart := range chartsToPublish {
		if !src.HasChart(chart) {
			return fmt.Errorf("chart %q does not exist in source dir %s", chart, options.chartDir)
		}
	}

	depGraph, err := dependency.NewGraph(src.LocalDependencies())
	if err != nil {
		return err
	}
	chartsToPublish = depGraph.WithDependents(chartsToPublish...)

	log.Debug().Msgf("Identified %d charts to publish: %s", len(chartsToPublish), strings.Join(chartsToPublish, ","))
	depGraph.TopoSort(chartsToPublish)

	_bucket, err := bucket.NewBucket(options.bucketName)
	_repo := repo.NewRepo(_bucket)
	defer func() {
		if err := _bucket.Close(); err != nil {
			log.Error().Msgf("error closing _bucket: %v", err)
		}
	}()

	scratchDir, err := app.Paths.CreateScratchDir("chart-publisher")
	if err != nil {
		return err
	}
	publisher, err := publish.NewPublisher(_repo, app.ShellRunner, scratchDir)
	if err != nil {
		return err
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			log.Error().Msgf("error closing publisher: %v", err)
		}
	}()

	for _, chartName := range chartsToPublish {
		chart, err := src.GetChart(chartName)
		if err != nil {
			return err
		}

		if err := chart.GenerateDocs(); err != nil {
			return err
		}
		if err := chart.BumpChartVersion(publisher.LastPublishedVersion(chartName)); err != nil {
			return err
		}
		if err := chart.BuildDependencies(); err != nil {
			return err
		}
		if err := chart.PackageChart(publisher.ChartDir()); err != nil {
			return err
		}
	}

	count, err := publisher.Publish(!options.dryRun)
	if err != nil {
		return err
	}

	log.Info().Msgf("Uploaded %d charts", count)

	return nil
}