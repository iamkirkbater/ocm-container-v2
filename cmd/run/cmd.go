package run

import (
	"context"
	"errors"
	"fmt"
	buildahCopiah "github.com/containers/buildah/copier"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/occ/pkg/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"
)

var (
	exec               string
	tag                string
	disableConsolePort bool
)

type fileSystemWrite interface {
	MkdirTemp(string, string) (string, error)
	RemoveAll(string) error
	Create(string) (*os.File, error)
	Fprintln(w io.Writer, a ...any) (n int, err error)
}

type osFileSystemWrite struct{}

func (osFileSystemWrite) MkdirTemp(dir string, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}
func (osFileSystemWrite) RemoveAll(path string) error          { return os.RemoveAll(path) }
func (osFileSystemWrite) Create(name string) (*os.File, error) { return os.Create(name) }
func (osFileSystemWrite) Fprintln(w io.Writer, a ...any) (n int, err error) {
	return fmt.Fprintln(w, a...)
}

type container interface {
	Inspect(ctx context.Context, nameOrID string, options *containers.InspectOptions) (*define.InspectContainerData, error)
	CopyFromArchive(ctx context.Context, nameOrID string, path string, reader io.Reader) (entities.ContainerCopyFunc, error)
}

type podmanContainer struct{}

func (podmanContainer) Inspect(ctx context.Context, nameOrID string, options *containers.InspectOptions) (*define.InspectContainerData, error) {
	return containers.Inspect(ctx, nameOrID, options)
}
func (podmanContainer) CopyFromArchive(ctx context.Context, nameOrID string, path string, reader io.Reader) (entities.ContainerCopyFunc, error) {
	return containers.CopyFromArchive(ctx, nameOrID, path, reader)
}

type copier interface {
	Get(root string, directory string, options buildahCopiah.GetOptions, globs []string, bulkWriter io.Writer) error
}

type builderCopier struct{}

func (builderCopier) Get(root string, directory string, options buildahCopiah.GetOptions, globs []string, bulkWriter io.Writer) error {
	return buildahCopiah.Get(root, directory, options, globs, bulkWriter)
}

type fileSystemRead interface {
	ReadDir(name string) ([]os.DirEntry, error)
	Stat(name string) (fs.FileInfo, error)
}

type osFileSystemRead struct{}

func (osFileSystemRead) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }
func (osFileSystemRead) Stat(name string) (fs.FileInfo, error)      { return os.Stat(name) }

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
	osFSr := osFileSystemRead{}
	osFSw := osFileSystemWrite{}

	configPath := config.Config.ConfigFileUsed()
	if _, err := osFSr.Stat(configPath); err != nil {
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
	s.Env = makeEnvMap(args, runtime.GOOS)
	s.Mounts = makeMounts(osFSr, configPath, homeDir, "/private/tmp", runtime.GOOS)
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
		err := copyPortmap(osFSw, podmanContainer{}, builderCopier{}, conn, createResponse.ID)
		if err != nil {
			log.Fatal("There was an error copying portmap file to the container", err)
		}
	}

	err = containers.Attach(conn, createResponse.ID, os.Stdin, os.Stdout, os.Stderr, nil, nil)
	if err != nil {
		log.Fatal("There was an error attaching to the container", err)
	}
}

func makeMounts(fs fileSystemRead, configPath string, homeDir string, macPrivateTempDir string, goos string) []specs.Mount {
	mountSlice := []specs.Mount{
		{
			Destination: "/root/.ssh/sockets",
			Type:        define.TypeTmpfs,
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
	if goos == "darwin" {
		agentLocation, err := macAgentLocation(fs, macPrivateTempDir)
		if err != nil {
			log.Fatal("Failed to retrieve agent location", err)
		}
		sshAgentMount = specs.Mount{
			Source:      macPrivateTempDir + "/" + agentLocation,
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
	if _, err := fs.Stat(homeDir + "/.config/gcloud"); err == nil {
		mountSlice = append(mountSlice, googleCliConfigMounts(homeDir)...)
	}

	// AWS token pull
	if _, err := fs.Stat(homeDir + "/.aws"); err == nil {
		mountSlice = append(mountSlice, awsCredentialsMounts(homeDir)...)
	}

	if opsUtilsDir := config.Config.GetString(config.OpsUtilsDirKey); opsUtilsDir != "" {
		opsUtilsDirMount := specs.Mount{
			Source:      opsUtilsDir,
			Destination: "/root/sop-utils",
			Type:        define.TypeBind,
		}
		if opsUtilsDirRw := config.Config.GetBool(config.OpsUtilsDirRWKey); opsUtilsDirRw {
			opsUtilsDirMount.Options = []string{"rw"}
		} else {
			opsUtilsDirMount.Options = []string{"ro"}
		}

		mountSlice = append(mountSlice, opsUtilsDirMount)
	}

	pagerdutyTokenFile := ".config/pagerduty-cli/config.json"
	if _, err := fs.Stat(homeDir + "/" + pagerdutyTokenFile); err == nil {
		mountSlice = append(mountSlice, specs.Mount{
			Source:      homeDir + "/" + pagerdutyTokenFile,
			Destination: "/root/" + pagerdutyTokenFile,
			Options:     []string{"ro"},
			Type:        define.TypeBind,
		})
	}
	return mountSlice
}

func makeEnvMap(args []string, goos string) map[string]string {
	envMap := map[string]string{}

	if ocmUser := config.Config.GetString(config.OCMUserKey); ocmUser != "" {
		envMap["USER"] = ocmUser
	}

	if offlineAccessToken := config.Config.GetString(config.OfflineAccessTokenKey); offlineAccessToken != "" {
		envMap["OFFLINE_ACCESS_TOKEN"] = offlineAccessToken
	}

	if ocmUrl := config.Config.GetString(config.OCMUrlKey); ocmUrl != "" {
		envMap["OCM_URL"] = ocmUrl
	}

	if len(args) > 0 {
		envMap["INITIAL_CLUSTER_LOGIN"] = args[0]
	}

	var sshAuthSock string
	if goos == "darwin" {
		sshAuthSock = "/tmp/ssh/Listeners"
	} else {
		sshAuthSock = "/tmp/ssh.sock"
	}
	envMap["SSH_AUTH_SOCK"] = sshAuthSock
	return envMap
}

func googleCliConfigMounts(homeDir string) []specs.Mount {
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

func awsCredentialsMounts(homeDir string) []specs.Mount {
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

func macAgentLocation(fs fileSystemRead, privateTempDir string) (string, error) {
	dirs, err := fs.ReadDir(privateTempDir)
	if err != nil {
		return "", err
	}

	if len(dirs) < 1 {
		return "", errors.New(fmt.Sprintf("no dirs found at %v", privateTempDir))
	}

	for _, dir := range dirs {
		if dirName := dir.Name(); strings.Contains(dirName, "com.apple.launchd") {
			return dirName, nil
		}
	}

	return "", errors.New(fmt.Sprintf("no dir found at %v containing com.apple.launchd", privateTempDir))
}

func copyPortmap(fs fileSystemWrite, container container, copier copier, conn context.Context, containerId string) error {

	tmpdir, err := fs.MkdirTemp("", "occ_portmaps")
	if err != nil {
		return fmt.Errorf("failed to create a tempdir for portmap: %v", err)
	}
	defer fs.RemoveAll(tmpdir)

	data, err := container.Inspect(conn, containerId, nil)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %v", err)
	}

	var hostPorts []string
	for _, host := range data.NetworkSettings.Ports["9999/tcp"] {
		hostPorts = append(hostPorts, host.HostPort)
	}

	portmapFile, err := fs.Create(tmpdir + "/portmap")
	if err != nil {
		return fmt.Errorf("failed to create portmap file: %v", err)
	}

	for _, port := range hostPorts {
		_, err := fs.Fprintln(portmapFile, port)
		if err != nil {
			return fmt.Errorf("failed to write host port to portmap file: %v", err)
		}
	}

	reader, writer := io.Pipe()
	hostCopy := func() error {
		defer writer.Close()
		getOptions := buildahCopiah.GetOptions{
			KeepDirectoryNames: true,
		}
		if err := copier.Get("/", "", getOptions, []string{portmapFile.Name()}, writer); err != nil {
			return fmt.Errorf("error copying portmap file from host: %v", err)
		}
		return nil
	}

	containerCopy := func() error {
		defer reader.Close()
		copyFunc, err := container.CopyFromArchive(conn, containerId, "/tmp", reader)
		if err != nil {
			return err
		}
		if err := copyFunc(); err != nil {
			return fmt.Errorf("error copying portmap file to container: %v", err)
		}
		return nil
	}

	if err := doCopy(hostCopy, containerCopy); err != nil {
		return fmt.Errorf("error copying portmap file from host to container: %v", err)
	}
	return nil
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
