package init

import (
	"bufio"
	"fmt"
	"github.com/openshift/occ/pkg/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

const (
	OCMUsernamePrompt        = `Provide your ocm user name`
	OfflineAccessTokenPrompt = `Provide your OCM Offline Access Token from https://cloud.redhat.com/openshift/token`
	OpsUtilsDirPrompt        = `(Optional) Provide your ops-sop/v4/utils directory.
This is an absolute path to any necessary scripts you wish to have automatically mounted into your container.
This is mounted in the "/root/sop_utils" directory in the container.`
	OpsUtilsDirRwPrompt = `Would you like the ops-sop directory to be mounted as readonly? [y/N]`
	WaitingForUserInput = `: `
)

func NewInitCmd() *cobra.Command {
	var initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initializes OCM container configuration",
		Long: `init will create a config file at ~/.config/occ/config.yaml.
If a config.yaml file already exists, you will be asked if you want to overwrite it or exit out.`,
		Run: setupConfig,
	}
	return initCmd
}

func setupConfig(*cobra.Command, []string) {
	reader := bufio.NewReader(os.Stdin)

	configPath := config.Config.ConfigFileUsed()
	if _, err := os.Stat(configPath); err == nil {
		if value := prompt(fmt.Sprintf("A config file already exists at %v, would you like to overwrite it? [y/N]", configPath), reader); strings.EqualFold(value, "y") {
			fmt.Println("The configuration file will be overwritten.")
			fmt.Println()
		} else {
			return
		}
	}

	ocmUser := prompt(OCMUsernamePrompt, reader)
	config.Config.Set(config.OCMUserKey, ocmUser)
	fmt.Println()

	offlineAccessToken := prompt(OfflineAccessTokenPrompt, reader)
	config.Config.Set(config.OfflineAccessTokenKey, offlineAccessToken)
	fmt.Println()

	opsUtilsDir := prompt(OpsUtilsDirPrompt, reader)
	config.Config.Set(config.OpsUtilsDirKey, opsUtilsDir)

	if opsUtilsDir != "" {
		fmt.Println()
		if value := prompt(OpsUtilsDirRwPrompt, reader); strings.EqualFold(value, "n") {
			config.Config.Set(config.OpsUtilsDirRWKey, true)
		} else {
			config.Config.Set(config.OpsUtilsDirRWKey, false)
		}
	}

	_, err := os.Stat(config.DefaultConfigFileLocation)
	if err != nil {
		err := os.MkdirAll(config.DefaultConfigFileLocation, 0755)
		if err != nil {
			log.Trace(err)
			log.Fatal("Failed to create necessary path for config file.")
		}
	}

	err = config.Config.WriteConfig()
	if err != nil {
		log.Trace(err)
		log.Fatal("Writing the config failed")
	}

	fmt.Printf("Config file has been written to %v", configPath)
}

type reader interface {
	ReadString(byte) (string, error)
}

func prompt(prompt string, reader reader) string {
	fmt.Println(prompt)
	fmt.Print(WaitingForUserInput)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSuffix(input, "\n")
	return input
}
