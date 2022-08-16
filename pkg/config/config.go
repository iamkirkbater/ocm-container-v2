package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	// Config is the exported configuration that any command can pull from
	Config *viper.Viper

	// The environment variable prefix of all environment variables bound to our command line flags.
	// For example, --number is bound to PREFIX_NUMBER.
	envPrefix = "OCC"
)

// InitConfig reads in config file and ENV variables if set.
func InitConfig(cmd *cobra.Command, cfgFile string) {
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
	v.ReadInConfig()

	// Set any necessary defaults for things that may not always be set via flags
	setDefaults(v)

	// Read in any environment variables that match flags
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()

	// bind any cobra flags into viper for a single source of truth
	bindFlags(cmd, v)

	Config = v
}

// sets the defaults for any configuration needed that may not be explicitly defined as a flag
func setDefaults(v *viper.Viper) {
	v.SetDefault("release-endpoint", "https://api.github.com/repos/iamkirkbater/ocm-container-v2/releases/latest")
	v.SetDefault("disable-update-checks", false)
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
