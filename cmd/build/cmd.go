package build

import (
	"context"
	"fmt"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/images"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/openshift/occ/pkg/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	buildahDefine "github.com/containers/buildah/define"
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
		Run:   build,
	}

	buildCmd.PersistentFlags().StringVarP(&tag, "tag", "t", "latest", "Container Image Tag")
	buildCmd.PersistentFlags().StringSliceVar(&buildArgs, "build-arg", []string{}, "Build Args to pass to container builder")

	return buildCmd
}

func build(cmd *cobra.Command, args []string) {
	socket := fmt.Sprintf("unix://%s", config.Config.GetString("podman-socket"))
	log.Trace("Using podman socket at: ", socket)
	conn, err := bindings.NewConnection(context.Background(), socket)
	if err != nil {
		log.Fatal("Error building connection to podman")
	}

	buildReport, err := images.Build(conn, []string{"./Dockerfile"}, entities.BuildOptions{
		BuildOptions: buildahDefine.BuildOptions{
			Args: map[string]string{
				"ARCH": "arm64",
			},
		},
	})
	if err != nil {
		log.Fatal("Error building occ container image: ", err)
	}

	fmt.Printf("%+v\n", buildReport)
}
