/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/planetscale/cli/internal/cmd/auth"
	"github.com/planetscale/cli/internal/cmd/backup"
	"github.com/planetscale/cli/internal/cmd/branch"
	"github.com/planetscale/cli/internal/cmd/connect"
	"github.com/planetscale/cli/internal/cmd/database"
	"github.com/planetscale/cli/internal/cmd/deployrequest"
	"github.com/planetscale/cli/internal/cmd/org"
	"github.com/planetscale/cli/internal/cmd/shell"
	"github.com/planetscale/cli/internal/cmd/snapshot"
	"github.com/planetscale/cli/internal/cmd/token"
	"github.com/planetscale/cli/internal/cmd/version"
	"github.com/planetscale/cli/internal/config"

	ps "github.com/planetscale/planetscale-go/planetscale"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:              "pscale",
	Short:            "A CLI for PlanetScale",
	Long:             `pscale is a CLI library for communicating with PlanetScale's API.`,
	TraverseChildren: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(ver, commit, buildDate string) error {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config",
		"", "Config file (default is $HOME/.config/planetscale/pscale.yml)")
	rootCmd.SilenceUsage = true

	v := version.Format(ver, commit, buildDate)
	rootCmd.SetVersionTemplate(v)
	rootCmd.Version = v
	rootCmd.Flags().Bool("version", false, "Show pscale version")

	cfg, err := config.New()
	if err != nil {
		return err
	}

	rootCmd.PersistentFlags().StringVar(&cfg.BaseURL,
		"api-url", ps.DefaultBaseURL, "The base URL for the PlanetScale API.")
	rootCmd.PersistentFlags().StringVar(&cfg.AccessToken,
		"api-token", cfg.AccessToken, "The API token to use for authenticating against the PlanetScale API.")

	if err := viper.BindPFlag("org", rootCmd.PersistentFlags().Lookup("org")); err != nil {
		return err
	}

	if err := viper.BindPFlag("database", rootCmd.PersistentFlags().Lookup("database")); err != nil {
		return err
	}

	if err := viper.BindPFlag("branch", rootCmd.PersistentFlags().Lookup("branch")); err != nil {
		return err
	}

	rootCmd.PersistentFlags().BoolVar(&cfg.OutputJSON, "json", false, "Show output as JSON")
	if err := viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json")); err != nil {
		return err
	}

	// service token flags. they are hidden for now.
	rootCmd.PersistentFlags().StringVar(&cfg.ServiceTokenName,
		"service-token-name", "", "The Service Token name for authenticating.")
	rootCmd.PersistentFlags().StringVar(&cfg.ServiceToken,
		"service-token", "", "Service Token for authenticating.")
	_ = rootCmd.PersistentFlags().MarkHidden("service-token-name")
	_ = rootCmd.PersistentFlags().MarkHidden("service-token")

	// We don't want to show the default value
	rootCmd.PersistentFlags().Lookup("api-token").DefValue = ""

	rootCmd.AddCommand(auth.AuthCmd(cfg))
	rootCmd.AddCommand(backup.BackupCmd(cfg))
	rootCmd.AddCommand(branch.BranchCmd(cfg))
	rootCmd.AddCommand(connect.ConnectCmd(cfg))
	rootCmd.AddCommand(database.DatabaseCmd(cfg))
	rootCmd.AddCommand(deployrequest.DeployRequestCmd(cfg))
	rootCmd.AddCommand(org.OrgCmd(cfg))
	rootCmd.AddCommand(shell.ShellCmd(cfg))
	rootCmd.AddCommand(snapshot.SnapshotCmd(cfg))
	rootCmd.AddCommand(token.TokenCmd(cfg))
	rootCmd.AddCommand(version.VersionCmd(cfg, ver, commit, buildDate))

	return rootCmd.Execute()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		defaultConfigDir, err := config.ConfigDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Order of preference for configuration files:
		// (1) $HOME/.config/planetscale
		viper.AddConfigPath(defaultConfigDir)
		viper.SetConfigName("pscale")
		viper.SetConfigType("yml")
	}

	viper.SetEnvPrefix("planetscale")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only handle errors when it's something unrelated to the config file not
			// existing.
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// Check for a project-local configuration file to merge in if the user
	// has not specified a config file
	if rootDir, err := config.RootGitRepoDir(); err == nil && cfgFile == "" {
		viper.AddConfigPath(rootDir)
		viper.SetConfigName(config.ProjectConfigFile())
		viper.MergeInConfig() // nolint:errcheck
	}

	postInitCommands(rootCmd.Commands())
}

// Hacky fix for getting Cobra required flags and Viper playing well together.
// See: https://github.com/spf13/viper/issues/397
func postInitCommands(commands []*cobra.Command) {
	for _, cmd := range commands {
		presetRequiredFlags(cmd)
		if cmd.HasSubCommands() {
			postInitCommands(cmd.Commands())
		}
	}
}

func presetRequiredFlags(cmd *cobra.Command) {
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		log.Fatalf("error binding flags: %v", err)
	}

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) && viper.GetString(f.Name) != "" {
			err = cmd.Flags().Set(f.Name, viper.GetString(f.Name))
			if err != nil {
				log.Fatalf("error setting flag %s: %v", f.Name, err)
			}
		}
	})
}
