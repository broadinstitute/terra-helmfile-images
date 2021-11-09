package cli

import (
	"github.com/spf13/cobra"
)

func newCiCommand(ctx *ThelmaContext) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.AddCommand(
		newPublishChartsCommand(ctx),
	)
	return cmd
}
