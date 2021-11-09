package cli

import "github.com/spf13/cobra"

const publishChartsHelpMessage = `Publishes Helm charts for Terra services`

func newPublishChartsCommand(ctx *ThelmaContext) *cobra.Command {
	return &cobra.Command{
		Use:           "publish-charts [options]",
		Short:         "Publishes Helm charts",
		Long:          publishChartsHelpMessage,
	}
}
