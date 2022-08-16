package version

import (
	"fmt"

	"github.com/openshift/ocm-container/pkg/updates"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewCheckCmd() *cobra.Command {
	// versionCmd represents the version command
	var checkCmd = &cobra.Command{
		Use:   "check",
		Short: "Checks for updates",
		Long:  `Checks the release API for updates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			updateResp, err := RunUpdateCheck()
			if err != nil {
				return err
			}

			if updateResp.HasAvailableUpdate() {
				fmt.Printf("An update is available at %s\n", updateResp.UpdateUrl)
				return nil
			}

			fmt.Println("No updates are available.")
			return nil
		},
	}

	return checkCmd
}

func RunUpdateCheck() (*updates.UpdateResponse, error) {
	updateConfig := &updates.UpdateConfig{
		GithubReleaseEndpoint: "https://api.github.com/repos/iamkirkbater/ocm-container-v2/releases/latest",
	}
	updateResp, err := updates.CheckForUpdates(updateConfig)
	if err != nil {
		log.Warn("Error fetching Updates.")
		return nil, err
	}
	return updateResp, nil
}
