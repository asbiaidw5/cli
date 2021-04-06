package database

import (
	"context"
	"fmt"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
	"github.com/planetscale/cli/internal/printer"

	"github.com/planetscale/planetscale-go/planetscale"

	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// ListCmd is the command for listing all databases for an authenticated user.
func ListCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List databases",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			web, err := cmd.Flags().GetBool("web")
			if err != nil {
				return err
			}

			if web {
				fmt.Println("🌐  Redirecting you to your databases list in your web browser.")
				err := browser.OpenURL(fmt.Sprintf("%s/%s", cmdutil.ApplicationURL, cfg.Organization))
				if err != nil {
					return err
				}
				return nil
			}

			client, err := cfg.NewClientFromConfig()
			if err != nil {
				return err
			}

			end := cmdutil.PrintProgress("Fetching databases...")
			defer end()
			databases, err := client.Databases.List(ctx, &planetscale.ListDatabasesRequest{
				Organization: cfg.Organization,
			})
			if err != nil {
				switch cmdutil.ErrCode(err) {
				case planetscale.ErrNotFound:
					return fmt.Errorf("%s does not exist\n", cmdutil.BoldBlue(cfg.Organization))
				case planetscale.ErrResponseMalformed:
					return cmdutil.MalformedError(err)
				default:
					return errors.Wrap(err, "error listing databases")
				}
			}

			end()

			if len(databases) == 0 && !cfg.OutputJSON {
				fmt.Println("No databases have been created yet.")
				return nil
			}

			err = printer.PrintOutput(cfg.OutputJSON, printer.NewDatabaseSlicePrinter(databases))
			if err != nil {
				return err
			}

			return nil
		},
		TraverseChildren: true,
	}

	cmd.Flags().BoolP("web", "w", false, "Open in your web browser")

	return cmd
}
