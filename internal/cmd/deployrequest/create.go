package deployrequest

import (
	"context"
	"fmt"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"
	"github.com/planetscale/planetscale-go/planetscale"

	"github.com/spf13/cobra"
)

// CreateCmd is the command for creating deploy requests.
func CreateCmd(ch *cmdutil.Helper) *cobra.Command {
	var flags struct {
		deployTo string
	}

	cmd := &cobra.Command{
		Use:   "create <database> <branch> [flags]",
		Short: "Create a deploy request from a branch",
		Args:  cmdutil.RequiredArgs("database", "branch"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			database := args[0]
			branch := args[1]

			client, err := ch.Config.NewClientFromConfig()
			if err != nil {
				return err
			}

			end := ch.Printer.PrintProgress(fmt.Sprintf("Request deploying of %s branch in %s...", cmdutil.BoldBlue(branch), cmdutil.BoldBlue(database)))
			defer end()

			dr, err := client.DeployRequests.Create(ctx, &planetscale.CreateDeployRequestRequest{
				Organization: ch.Config.Organization,
				Database:     database,
				Branch:       branch,
				IntoBranch:   flags.deployTo,
			})
			if err != nil {
				switch cmdutil.ErrCode(err) {
				case planetscale.ErrNotFound:
					return fmt.Errorf("database %s does not exist in %s\n",
						cmdutil.BoldBlue(database), cmdutil.BoldBlue(ch.Config.Organization))
				case planetscale.ErrResponseMalformed:
					return cmdutil.MalformedError(err)
				default:
					return err
				}
			}
			end()

			if ch.Printer.Format() == printer.Human {
				ch.Printer.Printf("Successfully requested deploy %s of %s into %s!\n",
					cmdutil.BoldBlue(dr.ID),
					cmdutil.BoldBlue(dr.Branch),
					cmdutil.BoldBlue(dr.IntoBranch))
				return nil
			}

			return ch.Printer.PrintResource(toDeployRequest(dr))
		},
	}

	cmd.PersistentFlags().StringVar(&flags.deployTo, "deploy-to", "main", "Branch to deploy the branch. By default it's set to 'main'")

	return cmd
}
