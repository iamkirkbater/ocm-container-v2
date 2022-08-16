package version

import (
	"fmt"

	"github.com/openshift/ocm-container/pkg/config"
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
		Run: func(cmd *cobra.Command, args []string) {
			updateResp, err := runUpdateCheck()
			if err != nil {
				fmt.Println(err)
				return
			}

			hasUpdates, err := updateResp.HasAvailableUpdate()
			if err != nil {
				fmt.Println(err)
				return
			}

			if hasUpdates {
				fmt.Printf("An update is available at %s\n", updateResp.UpdateUrl)
				return
			}
			fmt.Println("No updates available.")
		},
	}

	return checkCmd
}

func runUpdateCheck() (*updates.UpdateResponse, error) {
	updateConfig := &updates.UpdateConfig{
		GithubReleaseEndpoint: config.Config.GetString("release-endpoint"),
	}
	updateResp, err := updates.CheckForUpdates(updateConfig)
	if err != nil {
		log.Error("Error fetching Updates.")
		return nil, err
	}
	return updateResp, nil
}
