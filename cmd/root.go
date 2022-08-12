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

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/openshift/ocm-container/pkg/logcfg"
)

var (
	// The path to the config file, built later or set via var
	cfgFile string

	// the config file read in, for debug log printing
	readInConfig string

	// The environment variable prefix of all environment variables bound to our command line flags.
	// For example, --number is bound to PREFIX_NUMBER.
	envPrefix = "OCC"

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
			initConfig(cmd)
			logcfg.ToggleDebug(verbosity, cmd.Flags().Changed("verbosity"))
			if readInConfig != "" {
				log.Debug("Config read in from: ", readInConfig)
			}
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

	return rootCmd
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cmd *cobra.Command) {
	v := viper.New()
	if cfgFile != "" {
		// Use config file from the flag.
		v.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".projects" (without extension).
		v.AddConfigPath(home)
		v.SetConfigType("yaml")
		v.SetConfigName(".occ")
	}
	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err == nil {
		readInConfig = v.ConfigFileUsed()
	}

	v.SetEnvPrefix(envPrefix)

	v.AutomaticEnv() // read in environment variables that match

	bindFlags(cmd, v)
}

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent
		// keys with underscores, e.g. --favorite-color to PREFIX_FAVORITE_COLOR
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			v.BindEnv(f.Name, fmt.Sprintf("%s_%s", envPrefix, envVarSuffix))
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
