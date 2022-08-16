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
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/openshift/ocm-container/cmd/version"
	"github.com/openshift/ocm-container/pkg/config"
	"github.com/openshift/ocm-container/pkg/logcfg"
	"github.com/openshift/ocm-container/pkg/updates"
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
		Short: "OCM Container - A container-based workflow for SRE-ing OpenShift",
		Long:  `OCM Container v2 - This application contains the configuration manipulation and container runtime launcher for managing OpenShift clusters`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			config.InitConfig(cmd, cfgFile)

			logcfg.ToggleDebug(verbosity, cmd.Flags().Changed("verbosity"))

			// We're specifically checking this after the logcfg toggle so we don't always
			// print the debug line if there's a config file
			if config.Config.ConfigFileUsed() != "" {
				log.Debug("Config read in from: ", config.Config.ConfigFileUsed())
			}

			checkForUpdates(cmd)
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.occ.yaml)")

	// Defines the logging verbosity level.  Default is set to 'warn'.
	rootCmd.PersistentFlags().StringVarP(&verbosity, "verbosity", "v", "warn", "Log Level")

	rootCmd.AddCommand(version.NewVersionCmd())

	return rootCmd
}

// check for updates
func checkForUpdates(cmd *cobra.Command) {
	// if we're in the version check command, don't run the update check
	if cmd.Parent() != nil && cmd.Parent().Name() == "version" && cmd.Name() == "check" {
		return
	}

	// check to see if update checking is disabled and exit if it is
	if config.Config.GetBool("disable-update-checks") {
		log.Trace("Automatic Update Checking Disabled.")
		return
	}

	// check to see when the last time we checked for updates
	log.Trace("Getting last time we checked for updates.")
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		log.Trace("Could not get UserConfigDir while checking for updates")
		return
	}

	// if the config dir doesn't exist for occ
	occConfigDir := fmt.Sprintf("%s/%s", userConfigDir, "occ")
	if _, err := os.Stat(occConfigDir); os.IsNotExist(err) {
		log.Tracef("Creating config dir for occ at %s", occConfigDir)
		err := os.Mkdir(occConfigDir, 0750)
		if err != nil && !os.IsExist(err) {
			log.Trace("Error creating config dir")
			return
		}
		log.Trace("Config dir created")
	}

	// Create the time now object for comparison, as
	// well as the string time to write.
	now := time.Now().UTC()
	nowStr, err := now.MarshalText()
	if err != nil {
		log.Tracef("Cannot marshal time %s to text", now)
	}

	// if it's been more than 6 days or the file is empty
	// then check for updates. If the file doesn't exist,
	// exit without checking so we don't run the update check
	// every time if we fail to create the file. Only report
	// if there are updates, and print to stderr.
	updateTimeFile := fmt.Sprintf("%s/%s", occConfigDir, ".last-update-check")
	if _, err := os.Stat(updateTimeFile); os.IsNotExist(err) {
		log.Tracef("Creating update check file at %s", updateTimeFile)
		err = os.WriteFile(updateTimeFile, nowStr, 0660)
		if err != nil {
			log.Trace("Error writing current time to update check file. ", err)
			return
		}
		return
	}

	updateTimeRaw, err := os.ReadFile(updateTimeFile)
	if err != nil {
		log.Trace("There was an error reading the update time file. ", err)
	}
	updateTimeStr := strings.TrimSuffix(string(updateTimeRaw), "\n")
	updateTime, err := time.Parse(time.RFC3339, updateTimeStr)
	if err != nil {
		log.Trace("There was an error parsing the time from the update file. Rewriting with current time.")
		err = os.WriteFile(updateTimeFile, nowStr, 0660)
		if err != nil {
			log.Trace("Error writing current time to update check file. ", err)
			return
		}
	}

	err = os.WriteFile(updateTimeFile, nowStr, 0660)
	if err != nil {
		log.Trace("Error writing current time to update check file. ", err)
		return
	}

	duration := now.Sub(updateTime)
	log.Tracef("Last checked for updates %v ago", duration)

	if duration > (6 * 24 * time.Hour) {
		log.Trace("Checking for binary updates.")

		updateConfig := &updates.UpdateConfig{
			GithubReleaseEndpoint: config.Config.GetString("release-endpoint"),
		}
		updateResp, err := updates.CheckForUpdates(updateConfig)
		if err != nil {
			log.Trace("Error checking for updates: ", err)
			return
		}
		hasUpdates, err := updateResp.HasAvailableUpdate()
		if err != nil {
			log.Trace("Error checking for updates: ", err)
			return
		}
		if hasUpdates {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("Update: %s is available at %s", updateResp.LatestVersion, updateResp.UpdateUrl))
		}
	}
}
