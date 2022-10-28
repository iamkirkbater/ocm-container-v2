/*
Copyright © 2022 Kirk Bater kbater@redhat.com

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
	"time"

	initCmd "github.com/openshift/occ/cmd/init"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"go.szostok.io/version"
	"go.szostok.io/version/extension"
	"go.szostok.io/version/term"
	"go.szostok.io/version/upgrade"

	"github.com/openshift/occ/cmd/build"
	"github.com/openshift/occ/pkg/config"
	"github.com/openshift/occ/pkg/logcfg"
)

var (
	// The path to the config file, built later or set via var
	cfgFile string

	// The verbosity level for logs
	verbosity string
)

// NewRootCmd creates an instance of a new rootCmd for bootstrapping the application
func NewRootCmd() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "occ",
		Short: "OpenShift Command Center - A container-based workflow for SRE-ing OpenShift",
		Long:  `OpenShift Command Center - This application contains the configuration manipulation and container runtime launcher for managing OpenShift clusters`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			config.InitConfig(cmd, cfgFile)

			_ = logcfg.ToggleDebug(verbosity, cmd.Flags().Changed("verbosity"))

			// We're specifically checking this after the logcfg toggle so we don't always
			// print the debug line if there's a config file
			if config.Config.ConfigFileUsed() != "" {
				log.Debug("Config read in from: ", config.Config.ConfigFileUsed())
			}

			checkForUpdates(cmd, config.Config)
		},
		// Uncomment the following line if your bare application
		// has an action associated with it:
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("verbose ", verbosity)
			log.Info("Info Log")
			log.Debug("Debug Log")
			log.Trace("Trace Log")
			log.Error("testerr")
			log.Warn("warning")
		},
	}

	// Allows overwriting the default config file location
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", fmt.Sprintf("%s/config.yaml", config.DefaultConfigFileLocation), "config file location")

	// Defines the logging verbosity level.  Default is set to 'warn'.
	rootCmd.PersistentFlags().StringVarP(&verbosity, "verbosity", "v", "warn", "Log Level")

	rootCmd.AddCommand(extension.NewVersionCobraCmd(), initCmd.NewInitCmd())
	rootCmd.AddCommand(build.NewBuildCmd())

	return rootCmd
}

func checkForUpdates(cmd *cobra.Command, config *viper.Viper) {
	gh := upgrade.NewGitHubDetector(
		"iamkirkbater", "ocm-container-v2",
		upgrade.WithMinElapseTimeForRecheck(7*24*time.Hour),
	)

	rel, err := gh.LookForGreaterRelease(upgrade.LookForGreaterReleaseInput{
		CurrentVersion: version.Get().Version,
	})
	if err != nil {
		return
	}

	if !rel.Found {
		// no new version available
		return
	}
	if rel.ReleaseInfo.IsFromCache {
		// The time for re-checking for a new release has not elapsed yet,
		// so the cached version is returned.
		return
	}

	// Print the upgrade notice on a standard error channel (stderr).
	// As a result, output processing for a given command works properly even
	// if the upgrade notice is displayed.
	//
	// Use 'term.IsSmart' so that the renderer can disable colored output for non-tty output streams.
	out, err := gh.Render(rel.ReleaseInfo, term.IsSmart(cmd.OutOrStderr()))
	if err != nil {
		return
	}

	_, _ = fmt.Fprint(cmd.OutOrStderr(), out)
}
