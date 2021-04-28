package org

import (
	"context"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"

	"github.com/spf13/cobra"
)

func ListCmd(ch *cmdutil.Helper) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List the currently active organizations",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			client, err := ch.Client()
			if err != nil {
				return err
			}

			orgs, err := client.Organizations.List(ctx)
			if err != nil {
				return cmdutil.HandleError(err)
			}

			if len(orgs) == 0 && ch.Printer.Format() == printer.Human {
				ch.Printer.Printf("No organizations exist\n")
				return nil
			}

			return ch.Printer.PrintResource(toOrgs(orgs))
		},
	}

	return cmd
}
