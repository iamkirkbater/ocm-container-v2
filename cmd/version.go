package cmd

import (
	"fmt"

	"github.com/openshift/ocm-container/pkg/updates"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type versionOutput struct {
	Version string `json:"version" yaml:"version"`
	Build   string `json:"build" yaml:"build"`
}

func NewVersionCmd() *cobra.Command {
	// versionCmd represents the version command
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Gets version and build info for the command",
		Long:  `Gets version and build info for the command`,
		RunE: func(cmd *cobra.Command, args []string) error {
			updateConfig := &updates.UpdateConfig{
				GithubReleaseEndpoint: "https://api.github.com/repos/iamkirkbater/ocm-container-v2/releases/latest",
			}
			updateResp, err := updates.CheckForUpdates(updateConfig)
			if err != nil {
				log.Warn("Error fetching Updates.")
				return err
			}
			fmt.Printf("UpdateResp: %+v", updateResp)

			v := versionOutput{
				Version: updates.Version,
				Build:   updates.BuildCommit,
			}

			fmt.Printf("%+v\n", v)
			return nil
		},
	}

	return versionCmd
}
