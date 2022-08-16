package version

import (
	"fmt"

	"github.com/openshift/ocm-container/pkg/updates"
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
			v := versionOutput{
				Version: updates.Version,
				Build:   updates.BuildCommit,
			}

			fmt.Printf("%+v\n", v)
			return nil
		},
	}

	versionCmd.AddCommand(NewCheckCmd())

	return versionCmd
}
