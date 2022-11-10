package run

import (
	"context"
	"errors"
	"fmt"
	buildahCopiah "github.com/containers/buildah/copier"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/occ/pkg/config"
	"github.com/spf13/viper"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func init() {
	config.Config = viper.New()
	config.Config.Set(config.OCMUserKey, "testUser")
	config.Config.Set(config.OfflineAccessTokenKey, "testToken")
	config.Config.Set(config.OfflineAccessTokenKey, "testToken")
	config.Config.Set(config.OCMUrlKey, "testOcmUrl")
	config.Config.Set(config.OpsUtilsDirKey, "testOpsUtilsDir")
	config.Config.Set(config.OpsUtilsDirRWKey, true)
}

func TestMakeMounts(t *testing.T) {
	type test struct {
		name                 string
		testfs               fstest.MapFS
		expectedMounts       int
		expectGCPMount       bool
		expectAWSMount       bool
		expectOpsUtilsDir    bool
		expectOpsUtilsDirRw  bool
		expectPagerDutyToken bool
		goos                 string
	}

	tests := []test{
		{
			name: "All mounts",
			testfs: fstest.MapFS{
				"home_dir/.config/gcloud":                    {Mode: fs.ModeDir},
				"home_dir/.aws":                              {Mode: fs.ModeDir},
				"home_dir/.config/pagerduty-cli/config.json": {Data: []byte{}},
				"private/tmp/com.apple.launchd.test":         {Mode: fs.ModeDir},
			},
			expectedMounts:       12,
			expectGCPMount:       true,
			expectAWSMount:       true,
			expectOpsUtilsDir:    true,
			expectOpsUtilsDirRw:  true,
			expectPagerDutyToken: true,
			goos:                 "darwin",
		},
		{
			name: "No GCP mounts",
			testfs: fstest.MapFS{
				"home_dir/.aws": {Mode: fs.ModeDir},
				"home_dir/.config/pagerduty-cli/config.json": {Data: []byte{}},
				"private/tmp/com.apple.launchd.test":         {Mode: fs.ModeDir},
			},
			expectedMounts:       8,
			expectGCPMount:       false,
			expectAWSMount:       true,
			expectOpsUtilsDir:    true,
			expectOpsUtilsDirRw:  true,
			expectPagerDutyToken: true,
			goos:                 "darwin",
		},
		{
			name: "No AWS mounts",
			testfs: fstest.MapFS{
				"home_dir/.config/gcloud":                    {Mode: fs.ModeDir},
				"home_dir/.config/pagerduty-cli/config.json": {Data: []byte{}},
				"private/tmp/com.apple.launchd.test":         {Mode: fs.ModeDir},
			},
			expectedMounts:       10,
			expectGCPMount:       true,
			expectAWSMount:       false,
			expectOpsUtilsDir:    true,
			expectOpsUtilsDirRw:  true,
			expectPagerDutyToken: true,
			goos:                 "darwin",
		},
		{
			name: "No Ops Utils mounts",
			testfs: fstest.MapFS{
				"home_dir/.config/gcloud":                    {Mode: fs.ModeDir},
				"home_dir/.aws":                              {Mode: fs.ModeDir},
				"home_dir/.config/pagerduty-cli/config.json": {Data: []byte{}},
				"private/tmp/com.apple.launchd.test":         {Mode: fs.ModeDir},
			},
			expectedMounts:       11,
			expectGCPMount:       true,
			expectAWSMount:       true,
			expectOpsUtilsDir:    false,
			expectOpsUtilsDirRw:  true,
			expectPagerDutyToken: true,
			goos:                 "darwin",
		},
		{
			name: "All mounts Ops Utils read only",
			testfs: fstest.MapFS{
				"home_dir/.config/gcloud":                    {Mode: fs.ModeDir},
				"home_dir/.aws":                              {Mode: fs.ModeDir},
				"home_dir/.config/pagerduty-cli/config.json": {Data: []byte{}},
				"private/tmp/com.apple.launchd.test":         {Mode: fs.ModeDir},
			},
			expectedMounts:       12,
			expectGCPMount:       true,
			expectAWSMount:       true,
			expectOpsUtilsDir:    true,
			expectOpsUtilsDirRw:  false,
			expectPagerDutyToken: true,
			goos:                 "darwin",
		},
		{
			name: "No PagerDuty mount",
			testfs: fstest.MapFS{
				"home_dir/.config/gcloud":            {Mode: fs.ModeDir},
				"home_dir/.aws":                      {Mode: fs.ModeDir},
				"private/tmp/com.apple.launchd.test": {Mode: fs.ModeDir},
			},
			expectedMounts:       11,
			expectGCPMount:       true,
			expectAWSMount:       true,
			expectOpsUtilsDir:    true,
			expectOpsUtilsDirRw:  true,
			expectPagerDutyToken: false,
			goos:                 "darwin",
		},
		{
			name: "All mounts non-darwin",
			testfs: fstest.MapFS{
				"home_dir/.config/gcloud":                    {Mode: fs.ModeDir},
				"home_dir/.aws":                              {Mode: fs.ModeDir},
				"home_dir/.config/pagerduty-cli/config.json": {Data: []byte{}},
				"private/tmp/com.apple.launchd.test":         {Mode: fs.ModeDir},
			},
			expectedMounts:       12,
			expectGCPMount:       true,
			expectAWSMount:       true,
			expectOpsUtilsDir:    true,
			expectOpsUtilsDirRw:  true,
			expectPagerDutyToken: true,
			goos:                 "not-darwin",
		},
	}

	configPath := "config_path"
	homeDir := "home_dir"
	macPrivateTempDir := "private/tmp"

	for _, tc := range tests {
		if tc.expectOpsUtilsDir {
			config.Config.Set(config.OpsUtilsDirKey, "testOpsUtilsDir")
		} else {
			config.Config.Set(config.OpsUtilsDirKey, "")
		}

		if tc.expectOpsUtilsDirRw {
			config.Config.Set(config.OpsUtilsDirRWKey, true)
		} else {
			config.Config.Set(config.OpsUtilsDirRWKey, false)
		}

		t.Run(tc.name, func(t *testing.T) {
			mounts := makeMounts(tc.testfs, configPath, homeDir, macPrivateTempDir, tc.goos)

			if mountCount := len(mounts); mountCount != tc.expectedMounts {
				t.Fatalf("Unexpected number of mounts. Expected %v but got %v", tc.expectedMounts, mountCount)
			}

			mountMap := map[string]specs.Mount{}
			for _, mount := range mounts {
				mountMap[mount.Destination] = mount
			}

			var failures []string

			failures = append(failures, checkMount(mountMap, "", "/root/.ssh/sockets", []string{}, define.TypeTmpfs)...)
			failures = append(failures, checkMount(mountMap, configPath, "/root/.config/occ", []string{"ro"}, define.TypeBind)...)
			failures = append(failures, checkMount(mountMap, homeDir+"/.ssh", "/root/.ssh", []string{"ro"}, define.TypeBind)...)

			if tc.goos == "darwin" {
				failures = append(failures, checkMount(mountMap, macPrivateTempDir+"/com.apple.launchd.test", "/tmp/ssh", []string{"ro"}, define.TypeBind)...)
			} else {
				failures = append(failures, checkMount(mountMap, os.Getenv("SSH_AUTH_SOCK"), "/tmp/ssh.sock", []string{"ro"}, define.TypeBind)...)
			}

			if tc.expectGCPMount {
				failures = append(failures, checkMount(mountMap, homeDir+"/.config/gcloud/active_config", "/root/.config/gcloud/active_config_readonly", []string{"ro"}, define.TypeBind)...)
				failures = append(failures, checkMount(mountMap, homeDir+"/.config/gcloud/configurations/config_default", "/root/.config/gcloud/configurations/config_default_readonly", []string{"ro"}, define.TypeBind)...)
				failures = append(failures, checkMount(mountMap, homeDir+"/.config/gcloud/credentials.db", "/root/.config/gcloud/credentials_readonly.db", []string{"ro"}, define.TypeBind)...)
				failures = append(failures, checkMount(mountMap, homeDir+"/.config/gcloud/access_tokens.db", "/root/.config/gcloud/access_tokens_readonly.db", []string{"ro"}, define.TypeBind)...)
			}

			if tc.expectAWSMount {
				failures = append(failures, checkMount(mountMap, homeDir+"/.aws/credentials", "/root/.aws/credentials", []string{"ro"}, define.TypeBind)...)
				failures = append(failures, checkMount(mountMap, homeDir+"/.aws/config", "/root/.aws/config", []string{"ro"}, define.TypeBind)...)
			}

			if tc.expectOpsUtilsDir {
				var opsUtilRwOptions []string
				if tc.expectOpsUtilsDirRw {
					opsUtilRwOptions = append(opsUtilRwOptions, "rw")
				} else {
					opsUtilRwOptions = append(opsUtilRwOptions, "ro")
				}
				failures = append(failures, checkMount(mountMap, config.Config.GetString(config.OpsUtilsDirKey), "/root/sop-utils", opsUtilRwOptions, define.TypeBind)...)
			}

			if tc.expectPagerDutyToken {
				failures = append(failures, checkMount(mountMap, homeDir+"/.config/pagerduty-cli/config.json", "/root/.config/pagerduty-cli/config.json", []string{"ro"}, define.TypeBind)...)
			}

			if len(failures) > 0 {
				t.Fatalf(strings.Join(failures, "\n"))
			}
		})
	}
}

func checkMount(mountMap map[string]specs.Mount, source string, destination string, options []string, mountType string) []string {
	var failures []string
	if mount, ok := mountMap[destination]; ok {
		if mount.Source != source {
			failures = append(failures, fmt.Sprintf("For mount with destination %v, expected source to be %v but was %v", mount.Destination, "home_dir/.ssh", mount.Source))
		}
		if mount.Type != mountType {
			failures = append(failures, fmt.Sprintf("For mount with destination %v, expected type to be %v but was %v", mount.Destination, mountType, mount.Type))
		}
		for i := 0; i < len(options); i++ {
			if mount.Options[i] != options[i] {
				failures = append(failures, fmt.Sprintf("For mount with destination %v, expected options to contain %v", mount.Destination, strings.Join(options, ",")))
				break
			}
		}
	} else {
		failures = append(failures, fmt.Sprintf("Expected mount to exist with destination %v but none was found", destination))
	}
	return failures
}

func TestMakeEnvMap(t *testing.T) {
	type test struct {
		name             string
		goos             string
		expectedAuthSock string
	}

	tests := []test{
		{name: "darwin os", goos: "darwin", expectedAuthSock: "/tmp/ssh/Listeners"},
		{name: "non-darwin os", goos: "not darwin", expectedAuthSock: "/tmp/ssh.sock"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envMap := makeEnvMap([]string{"1234"}, tc.goos)
			var failures []string

			if val := envMap["USER"]; val != "testUser" {
				failures = append(failures, fmt.Sprintf("USER was %v, expected %v", val, "testUser"))
			}

			if val := envMap["OFFLINE_ACCESS_TOKEN"]; val != "testToken" {
				failures = append(failures, fmt.Sprintf("OFFLINE_ACCESS_TOKEN was %v, expected %v", val, "testToken"))
			}

			if val := envMap["OCM_URL"]; val != "testOcmUrl" {
				failures = append(failures, fmt.Sprintf("OCM_URL was %v, expected %v", val, "testOcmUrl"))
			}

			if val := envMap["INITIAL_CLUSTER_LOGIN"]; val != "1234" {
				failures = append(failures, fmt.Sprintf("INITIAL_CLUSTER_LOGIN was %v, expected %v", val, "1234"))
			}

			if val := envMap["SSH_AUTH_SOCK"]; val != tc.expectedAuthSock {
				failures = append(failures, fmt.Sprintf("INITIAL_CLUSTER_LOGIN was %v, expected %v", val, tc.expectedAuthSock))
			}

			if len(failures) > 0 {
				t.Fatalf(strings.Join(failures, "\n"))
			}
		})
	}
}

func TestMacAgentLocation(t *testing.T) {
	type test struct {
		name           string
		testfs         fstest.MapFS
		privateTempDir string
		expectedResult string
		expectedError  string
	}

	tests := []test{
		{
			name:           "Successfully finds agent location",
			testfs:         fstest.MapFS{"private/tmp/com.apple.launchd.test": {Mode: fs.ModeDir}},
			privateTempDir: "private/tmp",
			expectedResult: "com.apple.launchd.test",
			expectedError:  "",
		},
		{
			name:           "Fails to read private temp dir",
			testfs:         fstest.MapFS{},
			privateTempDir: "foo",
			expectedResult: "",
			expectedError:  "open foo: file does not exist",
		},
		{
			name:           "No dirs in private temp dir",
			testfs:         fstest.MapFS{"private/tmp": {Mode: fs.ModeDir}},
			privateTempDir: "private/tmp",
			expectedResult: "",
			expectedError:  "no dirs found at private/tmp",
		},
		{
			name:           "No dirs containing agent",
			testfs:         fstest.MapFS{"private/tmp/com.foo": {Mode: fs.ModeDir}},
			privateTempDir: "private/tmp",
			expectedResult: "",
			expectedError:  "no dir found at private/tmp containing com.apple.launchd",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := macAgentLocation(tc.testfs, tc.privateTempDir)
			if tc.expectedError != "" && err.Error() != tc.expectedError {
				t.Fatalf("Did not receive the expected error.\nExpected: %v\nActual: %v", tc.expectedError, err.Error())
			}
			if result != tc.expectedResult {
				t.Fatalf("Expected %v, but got %v", tc.expectedResult, result)
			}
		})
	}
}

func TestCopyPortMap(t *testing.T) {
	type test struct {
		name            string
		fileSystemWrite fileSystemWrite
		container       container
		copier          copier
		expected        string
	}

	tests := []test{
		{name: "Fails to create temp dir", fileSystemWrite: fsWriteFailMkdirTemp{}, expected: "failed to create a tempdir for portmap: fail"},
		{name: "Fails to inspect container", fileSystemWrite: fsWriteTest{}, container: containerFailInspect{}, expected: "failed to inspect container: fail"},
		{name: "Fails to create tmp portmap file", fileSystemWrite: fsWriteFailCreate{}, container: containerTest{}, expected: "failed to create portmap file: fail"},
		{name: "Fails to write portmap data", fileSystemWrite: fsWriteFailWritePortmap{}, container: containerTest{}, expected: "failed to write host port to portmap file: fail"},
		{name: "Fails to copy portmap file from host", fileSystemWrite: fsWriteTest{}, container: containerTest{}, copier: copierFailGet{}, expected: "error copying portmap file from host to container: 1 error occurred:\n\t* error copying portmap file from host: fail"},
		{name: "Fails to create copy function to copy portmap file to container", fileSystemWrite: fsWriteTest{}, container: containerFailCopyFromArchive{}, copier: copierTest{}, expected: "error copying portmap file from host to container: 1 error occurred:\n\t* fail"},
		{name: "Fails to create copy function that successfully copies portmap file to container", fileSystemWrite: fsWriteTest{}, container: containerFailCopyFromArchiveFunction{}, copier: copierTest{}, expected: "error copying portmap file from host to container: 1 error occurred:\n\t* error copying portmap file to container: fail"},
		{name: "Successfully copies file from host to container", fileSystemWrite: fsWriteTest{}, container: containerTest{}, copier: copierTest{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := copyPortmap(tc.fileSystemWrite, tc.container, tc.copier, nil, "")
			if tc.expected == "" && err != nil {
				t.Fatalf("Expected no error but got %v", err)
			}
			if tc.expected != "" {
				if err == nil {
					t.Fatalf("Expected %v but got no error", tc.expected)
				}
				if err.Error() != tc.expected {
					t.Fatalf("Expected %v but got %v", tc.expected, err)
				}
			}
		})
	}
	t.Cleanup(func() {
		err := os.RemoveAll("foo")
		if err != nil {
			t.Fatalf("Failed to clean up sample `foo` file.")
		}
	})
}

func TestDoCopy(t *testing.T) {
	type test struct {
		name     string
		funcA    func() error
		funcB    func() error
		expected string
	}

	tests := []test{
		{
			name: "Both functions return errors",
			funcA: func() error {
				return errors.New("A")
			},
			funcB: func() error {
				return errors.New("B")
			},
			expected: "2 errors occurred:\n\t* B\n\t* A",
		},
		{
			name: "First function returns an error",
			funcA: func() error {
				return errors.New("A")
			},
			funcB: func() error {
				return nil
			},
			expected: "1 error occurred:\n\t* A",
		},
		{
			name: "Second function returns an error",
			funcA: func() error {
				return nil
			},
			funcB: func() error {
				return errors.New("B")
			},
			expected: "1 error occurred:\n\t* B",
		},
		{
			name: "Neither function returns an error",
			funcA: func() error {
				return nil
			},
			funcB: func() error {
				return nil
			},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := doCopy(tc.funcA, tc.funcB)
			if tc.expected != "" && result.Error() != tc.expected {
				t.Fatalf("Expected %v but got %v", tc.expected, result)
			}
			if tc.expected == "" && result != nil {
				t.Fatalf("Expected no errors but got %v", result)
			}
		})
	}
}

// fileSystemWrite impls
type fsWriteTest struct{}

func (fsWriteTest) RemoveAll(string) error { return nil }
func (fsWriteTest) Create(string) (*os.File, error) {
	return os.Create("foo")
}
func (fsWriteTest) MkdirTemp(string, string) (string, error)      { return "tmpfile", nil }
func (fsWriteTest) Fprintln(io.Writer, ...any) (n int, err error) { return 0, nil }

type fsWriteFailMkdirTemp struct{}

func (fsWriteFailMkdirTemp) RemoveAll(string) error                        { panic(nil) }
func (fsWriteFailMkdirTemp) Create(string) (*os.File, error)               { panic(nil) }
func (fsWriteFailMkdirTemp) MkdirTemp(string, string) (string, error)      { return "", errors.New("fail") }
func (fsWriteFailMkdirTemp) Fprintln(io.Writer, ...any) (n int, err error) { panic(nil) }

type fsWriteFailCreate struct{}

func (fsWriteFailCreate) RemoveAll(string) error                        { return nil }
func (fsWriteFailCreate) Create(string) (*os.File, error)               { return nil, errors.New("fail") }
func (fsWriteFailCreate) MkdirTemp(string, string) (string, error)      { return "tmpfile", nil }
func (fsWriteFailCreate) Fprintln(io.Writer, ...any) (n int, err error) { panic(nil) }

type fsWriteFailWritePortmap struct{}

func (fsWriteFailWritePortmap) RemoveAll(string) error                   { return nil }
func (fsWriteFailWritePortmap) Create(string) (*os.File, error)          { return &os.File{}, nil }
func (fsWriteFailWritePortmap) MkdirTemp(string, string) (string, error) { return "tmpfile", nil }
func (fsWriteFailWritePortmap) Fprintln(io.Writer, ...any) (n int, err error) {
	return 0, errors.New("fail")
}

// container impls
type containerTest struct{}

func (containerTest) Inspect(context.Context, string, *containers.InspectOptions) (*define.InspectContainerData, error) {
	return &define.InspectContainerData{
		NetworkSettings: &define.InspectNetworkSettings{
			Ports: map[string][]define.InspectHostPort{"9999/tcp": {{HostPort: "12345"}}},
		},
	}, nil
}
func (containerTest) CopyFromArchive(context.Context, string, string, io.Reader) (entities.ContainerCopyFunc, error) {
	return func() error {
		return nil
	}, nil
}

type containerFailInspect struct{}

func (containerFailInspect) Inspect(context.Context, string, *containers.InspectOptions) (*define.InspectContainerData, error) {
	return nil, errors.New("fail")
}
func (containerFailInspect) CopyFromArchive(context.Context, string, string, io.Reader) (entities.ContainerCopyFunc, error) {
	panic(nil)
}

type containerFailCopyFromArchive struct{}

func (containerFailCopyFromArchive) Inspect(context.Context, string, *containers.InspectOptions) (*define.InspectContainerData, error) {
	return &define.InspectContainerData{
		NetworkSettings: &define.InspectNetworkSettings{
			Ports: map[string][]define.InspectHostPort{"9999/tcp": {{HostPort: "12345"}}},
		},
	}, nil
}
func (containerFailCopyFromArchive) CopyFromArchive(context.Context, string, string, io.Reader) (entities.ContainerCopyFunc, error) {
	return nil, errors.New("fail")
}

type containerFailCopyFromArchiveFunction struct{}

func (containerFailCopyFromArchiveFunction) Inspect(context.Context, string, *containers.InspectOptions) (*define.InspectContainerData, error) {
	return &define.InspectContainerData{
		NetworkSettings: &define.InspectNetworkSettings{
			Ports: map[string][]define.InspectHostPort{"9999/tcp": {{HostPort: "12345"}}},
		},
	}, nil
}
func (containerFailCopyFromArchiveFunction) CopyFromArchive(context.Context, string, string, io.Reader) (entities.ContainerCopyFunc, error) {
	return func() error {
		return errors.New("fail")
	}, nil
}

// copier impls
type copierTest struct{}

func (copierTest) Get(string, string, buildahCopiah.GetOptions, []string, io.Writer) error {
	return nil
}

type copierFailGet struct{}

func (copierFailGet) Get(string, string, buildahCopiah.GetOptions, []string, io.Writer) error {
	return errors.New("fail")
}
