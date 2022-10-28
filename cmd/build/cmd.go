package build

import (
	"context"
	"fmt"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/images"
	"github.com/containers/podman/v4/pkg/domain/entities"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	// What to tag the container image as
	tag string

	// Build Args to pass to container builder
	buildArgs []string
)

// NewBuildCmd creates an instance of a new buildCmd for bootstrapping the application
func NewBuildCmd() *cobra.Command {
	var buildCmd = &cobra.Command{
		Use:   "build",
		Short: "builds the container image",
		Long:  `Builds the container image to be used. This allows SREs to add additional functionality to occ's container image`,
		Run: func(cmd *cobra.Command, args []string) {
			conn, err := bindings.NewConnection(context.Background(), "unix:///Users/kbater/.local/share/containers/podman/machine/podman-machine-default/podman.sock")
			if err != nil {
				log.Fatal("Error building connection to podman")
			}

			buildReport, err := images.Build(conn, []string{".Dockerfile"}, entities.BuildOptions{})
			if err != nil {
				log.Fatal("Error building occ container image: ", err)
			}

			fmt.Sprintf("%+v", buildReport)
		},
	}

	buildCmd.PersistentFlags().StringVarP(&tag, "tag", "t", "latest", "Container Image Tag")
	buildCmd.PersistentFlags().StringSliceVar(&buildArgs, "build-arg", []string{}, "Build Args to pass to container builder")

	return buildCmd
}
