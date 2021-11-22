package cli

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/mirror"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/cli/builders"
	"github.com/spf13/cobra"
	"path"
)

const chartsImportHelpMessage = `Imports charts from public Helm repositories into the terra-helm-third-party-repo`
const chartsImportDefaultBucketName = "terra-helm-thirdparty"
const chartsImportDefaultConfigFile = ".third-party-charts.yaml"

type chartsImportOptions struct {
	configFile string
	bucketName string
	dryRun     bool
}

var chartsImportFlagNames = struct {
	configFile   string
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
		Use:   "import [options] [CHART1] [CHART2] ...",
		Short: "Imports third-party Helm charts into the terra-helm-thirdparty repo",
		Long:  chartsImportHelpMessage,
	}

	cobraCommand.Flags().StringVar(&options.configFile, chartsImportFlagNames.configFile, path.Join("$THELMA_HOME", chartsImportDefaultConfigFile), "Path to import config file")
	cobraCommand.Flags().StringVar(&options.bucketName, chartsImportFlagNames.bucketName, chartsImportDefaultBucketName, "Publish charts to custom GCS bucket")
	cobraCommand.Flags().BoolVarP(&options.dryRun, chartsImportFlagNames.dryRun, "n", false, "Dry run (don't actually update Helm repo)")

	cobraCommand.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed(chartsImportFlagNames.configFile) {
			expanded, err := expandAndVerifyExists(options.configFile, "configFile")
			if err != nil {
				return err
			}
			options.configFile = expanded
		} else {
			options.configFile = path.Join(ctx.app.Config.Home(), chartsImportDefaultConfigFile)
		}

		return nil
	}

	cobraCommand.RunE = func(cmd *cobra.Command, args []string) error {
		return importCharts(&options, ctx.app)
	}

	return &chartsImportCLI{
		ctx:          ctx,
		cobraCommand: cobraCommand,
		options:      &options,
	}
}

func importCharts(options *chartsImportOptions, app *app.ThelmaApp) error {
	pb, err := builders.Publisher(app, options.bucketName, options.dryRun)
	if err != nil {
		return err
	}
	defer pb.CloseWarn()

	_mirror, err := mirror.NewMirror(pb.Publisher(), app.ShellRunner, options.configFile)
	if err != nil {
		return err
	}

	return _mirror.ImportToMirror()
}
