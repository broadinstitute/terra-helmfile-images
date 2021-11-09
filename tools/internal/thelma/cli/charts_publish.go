package cli

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const chartsPublishHelpMessage = `Publishes Helm charts for Terra services`

type chartsPublishCLI struct {
	cobraCommand *cobra.Command
}

func newChartsPublishCLI(ctx *ThelmaContext) *chartsPublishCLI {
	cobraCommand := &cobra.Command{
		Use:   "publish [options]",
		Short: "Publishes Helm charts",
		Long:  chartsPublishHelpMessage,
	}
	cobraCommand.RunE = func(cmd *cobra.Command, args []string) error {
		log.Info().Msgf("TODO")
		return nil
	}
	return &chartsPublishCLI{
		cobraCommand: cobraCommand,
	}
}
