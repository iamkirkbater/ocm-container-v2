package run

import (
	"context"
	"errors"
	"fmt"
	buildahCopiah "github.com/containers/buildah/copier"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/opencontainers/runtime-spec/specs-go"
	initCmd "github.com/openshift/occ/cmd/init"
	"github.com/openshift/occ/pkg/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"os"
	"runtime"
	"strings"
)

var (
	exec               string
	tag                string
	disableConsolePort bool
)

func NewRunCmd() *cobra.Command {
	var runCmd = &cobra.Command{
		Use:   "run [cluster_id]",
		Short: "Runs an OCM container instance",
		Long:  `Run will start up an OCM container instance using a given configuration file.`,
		Args:  cobra.MaximumNArgs(1),
		Run:   runContainer,
	}

	runCmd.PersistentFlags().StringVarP(&exec, "exec", "e", "", "Path (in-container) to a script to run on-cluster and exit")
	runCmd.PersistentFlags().StringVarP(&tag, "tag", "t", "latest", "Sets the image tag to use")
	runCmd.PersistentFlags().BoolVarP(&disableConsolePort, "disable-console-port", "d", false, "Disable automatic cluster console port mapping")

	return runCmd
}

func runContainer(_ *cobra.Command, args []string) {
	configPath := config.Config.ConfigFileUsed()
	if _, err := os.Stat(configPath); err != nil {
		log.Fatalf(`Cannot find config file at %v. Run occ init to create one.`, configPath)
	}

	homeDir, _ := os.UserHomeDir()
	socket := config.Config.GetString("podman-socket")
	log.Trace("Using podman socket at: ", socket)
	conn, err := bindings.NewConnection(context.Background(), socket)
	if err != nil {
		log.Trace(err)
		log.Fatal("Error building connection to podman")
	}

	s := specgen.NewSpecGenerator("localhost/ocm-container:"+tag, false)
	s.Stdin = true
	s.Terminal = true
	s.Remove = true
	s.Privileged = true
	s.Env = makeEnvMap(args)
	s.Mounts = makeMounts(configPath, homeDir)
	s.PublishExposedPorts = !disableConsolePort
	createResponse, err := containers.CreateWithSpec(conn, s, nil)
	if err != nil {
		log.Trace(err)
		log.Fatal("Failed to create container")
	}

	if err := containers.Start(conn, createResponse.ID, nil); err != nil {
		log.Trace(err)
		log.Fatal("Failed to start container")
	}

	if !disableConsolePort {
		copyPortmap(conn, createResponse.ID)
	}

	err = containers.Attach(conn, createResponse.ID, os.Stdin, os.Stdout, os.Stderr, nil, nil)
	if err != nil {
		log.Fatal("There was an error attaching to the container", err)
	}
}

func makeMounts(configPath string, homeDir string) []specs.Mount {
	mountSlice := []specs.Mount{
		{
			Destination: "/root/.ssh/sockets",
			Type:        "tmpfs",
		},
		{
			Source:      configPath,
			Destination: "/root/.config/occ",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		},
		{
			Source:      homeDir + "/.ssh",
			Destination: "/root/.ssh",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		},
	}

	var sshAgentMount specs.Mount
	if runtime.GOOS == "darwin" {
		agentLocation, err := macAgentLocation()
		if err != nil {
			log.Fatal("Failed to retrieve agent location", err)
		}
		sshAgentMount = specs.Mount{
			Source:      "/private/tmp/" + agentLocation,
			Destination: "/tmp/ssh",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		}
	} else {
		sshAgentMount = specs.Mount{
			Source:      os.Getenv("SSH_AUTH_SOCK"),
			Destination: "/tmp/ssh.sock",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		}
	}
	mountSlice = append(mountSlice, sshAgentMount)

	// Google Cloud CLI config mounting
	if _, err := os.Stat(homeDir + "/.config/gcloud"); err == nil {
		mountSlice = append(mountSlice, googleCliConfigMounts()...)
	}

	// AWS token pull
	if _, err := os.Stat(homeDir + "/.aws"); err == nil {
		mountSlice = append(mountSlice, awsCredentialsMounts()...)
	}

	if opsUtilsDir := config.Config.GetString(initCmd.OpsUtilsDirKey); opsUtilsDir != "" {
		opsUtilsDirMount := specs.Mount{
			Source:      opsUtilsDir,
			Destination: "/root/sop-utils",
			Type:        define.TypeBind,
		}
		if opsUtilsDirRw := config.Config.GetBool(initCmd.OpsUtilsDirRWKey); opsUtilsDirRw == true {
			opsUtilsDirMount.Options = []string{"rw"}
		} else {
			opsUtilsDirMount.Options = []string{"ro"}
		}

		mountSlice = append(mountSlice, opsUtilsDirMount)
	}

	pagerdutyTokenFile := ".config/pagerduty-cli/config.json"
	if _, err := os.Stat(homeDir + pagerdutyTokenFile); err == nil {
		mountSlice = append(mountSlice, specs.Mount{
			Source:      homeDir + "/" + pagerdutyTokenFile,
			Destination: "/root/" + pagerdutyTokenFile,
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		})
	}
	return mountSlice
}

func makeEnvMap(args []string) map[string]string {
	envMap := map[string]string{}

	if ocmUser := config.Config.GetString(initCmd.OCMUserKey); ocmUser != "" {
		envMap["USER"] = ocmUser
	}

	if offlineAccessToken := config.Config.GetString(initCmd.OfflineAccessTokenKey); offlineAccessToken != "" {
		envMap["OFFLINE_ACCESS_TOKEN"] = offlineAccessToken
	}

	if ocmUrl := config.Config.GetString("ocm_url"); ocmUrl != "" {
		envMap["OCM_URL"] = ocmUrl
	}

	if len(args) > 0 {
		envMap["INITIAL_CLUSTER_LOGIN"] = args[0]
	}

	var sshAuthSock string
	if runtime.GOOS == "darwin" {
		sshAuthSock = "/tmp/ssh/Listeners"
	} else {
		sshAuthSock = "/tmp/ssh.sock"
	}
	envMap["SSH_AUTH_SOCK"] = sshAuthSock
	return envMap
}

func googleCliConfigMounts() []specs.Mount {
	homeDir, _ := os.UserHomeDir()
	return []specs.Mount{
		{
			Source:      homeDir + "/.config/gcloud/active_config",
			Destination: "/root/.config/gcloud/active_config_readonly",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		}, {
			Source:      homeDir + "/.config/gcloud/configurations/config_default",
			Destination: "/root/.config/gcloud/configurations/config_default_readonly",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		}, {
			Source:      homeDir + "/.config/gcloud/credentials.db",
			Destination: "/root/.config/gcloud/credentials_readonly.db",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		}, {
			Source:      homeDir + "/.config/gcloud/access_tokens.db",
			Destination: "/root/.config/gcloud/access_tokens_readonly.db",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		},
	}
}

func awsCredentialsMounts() []specs.Mount {
	homeDir, _ := os.UserHomeDir()
	return []specs.Mount{
		{
			Source:      homeDir + "/.aws/credentials",
			Destination: "/root/.aws/credentials",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		},
		{
			Source:      homeDir + "/.aws/config",
			Destination: "/root/.aws/config",
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		},
	}
}

func macAgentLocation() (string, error) {
	dirs, err := os.ReadDir("/private/tmp")
	if err != nil {
		return "", err
	}

	if len(dirs) < 1 {
		return "", errors.New("no dirs found at /private/tmp")
	}

	for _, dir := range dirs {
		if dirName := dir.Name(); strings.Contains(dirName, "com.apple.launchd") {
			return dirName, nil
		}
	}

	return "", errors.New("no dir found at /private/tmp containing com.apple.launchd")
}

func copyPortmap(conn context.Context, containerId string) {
	tmpdir, err := os.MkdirTemp("", "occ_portmaps")
	if err != nil {
		log.Fatal("Failed to create a tempdir for portmap")
	}
	defer os.RemoveAll(tmpdir)

	data, err := containers.Inspect(conn, containerId, nil)
	if err != nil {
		log.Fatal("Failed to inspect container")
	}

	var hostPorts []string
	for _, host := range data.NetworkSettings.Ports["9999/tcp"] {
		hostPorts = append(hostPorts, host.HostPort)
	}

	portmapFile, err := os.Create(tmpdir + "/portmap")
	if err != nil {
		log.Fatal("Failed to create portmap file")
	}

	for _, port := range hostPorts {
		_, err := fmt.Fprintln(portmapFile, port)
		if err != nil {
			log.Fatal("Failed to write host port to portmap file")
		}
	}

	reader, writer := io.Pipe()
	hostCopy := func() error {
		defer writer.Close()
		getOptions := buildahCopiah.GetOptions{
			KeepDirectoryNames: true,
		}
		if err := buildahCopiah.Get("/", "", getOptions, []string{portmapFile.Name()}, writer); err != nil {
			return fmt.Errorf("error copying portmap file from host")
		}
		return nil
	}

	containerCopy := func() error {
		defer reader.Close()
		copyFunc, err := containers.CopyFromArchive(conn, containerId, "/tmp", reader)
		if err != nil {
			return err
		}
		if err := copyFunc(); err != nil {
			return err
		}
		return nil
	}

	if err := doCopy(hostCopy, containerCopy); err != nil {
		log.Fatal("Error copying portmap file to host.")
	}
}

// Copied from https://github.com/containers/podman/blob/main/cmd/podman/containers/cp.go#L113
func doCopy(hostCopyFunc func() error, containerCopyFunc func() error) error {
	errChan := make(chan error)
	go func() {
		errChan <- hostCopyFunc()
	}()
	var copyErrors []error
	copyErrors = append(copyErrors, containerCopyFunc())
	copyErrors = append(copyErrors, <-errChan)
	return errorhandling.JoinErrors(copyErrors)
}
