package deployrequest

import (
	"errors"
	"fmt"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

// ListCmd is the command for listing deploy requests.
func ListCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List deploy requests",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			web, err := cmd.Flags().GetBool("web")
			if err != nil {
				return err
			}

			if web {
				fmt.Println("🌐  Redirecting you to your deploy-requests list in your web browser.")
				err := browser.OpenURL(fmt.Sprintf("%s/%s", cmdutil.ApplicationURL, cfg.Organization))
				if err != nil {
					return err
				}
				return nil
			}

			_, err = cfg.NewClientFromConfig()
			if err != nil {
				return err
			}

			return errors.New("not implemented yet")
		},
		TraverseChildren: true,
	}

	cmd.Flags().BoolP("web", "w", false, "Open in your web browser")

	return cmd
}
