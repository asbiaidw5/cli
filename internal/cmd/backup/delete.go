package backup

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/printer"

	"github.com/planetscale/planetscale-go/planetscale"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"
)

func DeleteCmd(ch *cmdutil.Helper) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <database> <branch> <backup>",
		Short:   "Delete a branch backup",
		Args:    cmdutil.RequiredArgs("database", "branch", "backup"),
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			database := args[0]
			branch := args[1]
			backup := args[2]

			client, err := ch.Config.NewClientFromConfig()
			if err != nil {
				return err
			}

			if !force {
				if ch.Printer.Format() != printer.Human {
					return fmt.Errorf("Cannot delete backup with the output format %q (run with -force to override)", ch.Printer.Format())
				}

				confirmationName := fmt.Sprintf("%s/%s/%s", database, branch, backup)
				if !cmdutil.IsTTY {
					return fmt.Errorf("Cannot confirm deletion of backup %q (run with -force to override)", confirmationName)
				}

				confirmationMessage := fmt.Sprintf("%s %s %s", cmdutil.Bold("Please type"), cmdutil.BoldBlue(confirmationName), cmdutil.Bold("to confirm:"))

				prompt := &survey.Input{
					Message: confirmationMessage,
				}

				var userInput string
				err := survey.AskOne(prompt, &userInput)
				if err != nil {
					if err == terminal.InterruptErr {
						os.Exit(0)
					} else {
						return err
					}
				}

				// If the confirmations don't match up, let's return an error.
				if userInput != confirmationName {
					return errors.New("Incorrect backup name entered, skipping backup deletion...")
				}
			}

			end := ch.Printer.PrintProgress(fmt.Sprintf("Deleting backup %s from %s", cmdutil.BoldBlue(backup), cmdutil.BoldBlue(branch)))
			defer end()

			err = client.Backups.Delete(ctx, &planetscale.DeleteBackupRequest{
				Organization: ch.Config.Organization,
				Database:     database,
				Branch:       branch,
				Backup:       backup,
			})
			if err != nil {
				switch cmdutil.ErrCode(err) {
				case planetscale.ErrNotFound:
					return fmt.Errorf("backup %s does not exist in branch %s of %s (organization: %s)\n",
						cmdutil.BoldBlue(backup), cmdutil.BoldBlue(branch), cmdutil.BoldBlue(database), cmdutil.BoldBlue(ch.Config.Organization))
				case planetscale.ErrResponseMalformed:
					return cmdutil.MalformedError(err)
				default:
					return err
				}
			}

			end()

			if ch.Printer.Format() == printer.Human {
				ch.Printer.Printf("Backup %s was successfully deleted from %s!\n",
					cmdutil.BoldBlue(backup), cmdutil.BoldBlue(branch))
				return nil
			}

			return ch.Printer.PrintResource(
				map[string]string{
					"result": "backup deleted",
					"backup": backup,
					"branch": branch,
				},
			)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Delete a backup without confirmation")
	return cmd
}
