package run

import (
	"errors"
	"fmt"
	buildahCopiah "github.com/containers/buildah/copier"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/golang/mock/gomock"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/occ/cmd/mocks"
	"github.com/openshift/occ/pkg/config"
	"github.com/spf13/viper"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// TODOs:
// 1. Look for any use of fstest and replace with gomock (maybe?)
// 2. Look for any structs that are just implementing impls and replace with go mock (bottom of file)

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

type mockData struct {
	data  any
	err   error
	times int
}

func TestCopyPortmap(t *testing.T) {
	type test struct {
		name            string
		fileSystemWrite FileSystemWrite
		mkTmpDir        mockData
		removeAll       mockData
		create          mockData
		fprintln        mockData
		container       Container
		inspect         mockData
		copyFromArchive mockData
		copier          Copier
		get             mockData
		expected        string
	}

	inspectData := &define.InspectContainerData{
		NetworkSettings: &define.InspectNetworkSettings{
			Ports: map[string][]define.InspectHostPort{"9999/tcp": {{HostPort: "12345"}}},
		},
	}

	fileInfo, err := os.Create("foo")
	if err != nil {
		t.Fatalf("Failed to create test file")
	}

	tests := []test{
		{
			name:     "Fails to create temp dir",
			mkTmpDir: mockData{data: "", err: errors.New("fail"), times: 1},
			fprintln: mockData{data: 0},
			expected: "failed to create a tempdir for portmap: fail",
		},
		{
			name:      "Fails to inspect Container",
			mkTmpDir:  mockData{data: "tmpdir", times: 1},
			removeAll: mockData{times: 1},
			fprintln:  mockData{data: 0},
			inspect:   mockData{err: errors.New("fail"), times: 1},
			expected:  "failed to inspect Container: fail",
		},
		{
			name:      "Fails to create tmp portmap file",
			mkTmpDir:  mockData{data: "tmpdir", times: 1},
			removeAll: mockData{times: 1},
			fprintln:  mockData{data: 0},
			inspect:   mockData{data: inspectData, times: 1},
			create:    mockData{err: errors.New("fail"), times: 1},
			expected:  "failed to create portmap file: fail",
		},
		{
			name:      "Fails to write portmap data",
			mkTmpDir:  mockData{data: "tmpdir", times: 1},
			removeAll: mockData{times: 1},
			fprintln:  mockData{data: 0, err: errors.New("fail"), times: 1},
			inspect:   mockData{data: inspectData, times: 1},
			create:    mockData{data: fileInfo, times: 1},
			expected:  "failed to write host port to portmap file: fail",
		},
		{
			name:            "Fails to copy portmap file from host",
			mkTmpDir:        mockData{data: "tmpdir", times: 1},
			removeAll:       mockData{times: 1},
			fprintln:        mockData{data: 0, times: 1},
			inspect:         mockData{data: inspectData, times: 1},
			copyFromArchive: mockData{data: func() error { return nil }, err: nil, times: 1},
			create:          mockData{data: fileInfo, times: 1},
			get:             mockData{err: errors.New("fail"), times: 1},
			expected:        "error copying portmap file from host to Container: 1 error occurred:\n\t* error copying portmap file from host: fail",
		},
		{
			name:            "Fails to create copy function to copy portmap file to Container",
			mkTmpDir:        mockData{data: "tmpdir", times: 1},
			removeAll:       mockData{times: 1},
			fprintln:        mockData{data: 0, times: 1},
			inspect:         mockData{data: inspectData, times: 1},
			copyFromArchive: mockData{data: func() error { return nil }, err: errors.New("fail"), times: 1},
			create:          mockData{data: fileInfo, times: 1},
			get:             mockData{err: nil, times: 1},
			expected:        "error copying portmap file from host to Container: 1 error occurred:\n\t* fail",
		},
		{
			name:            "Fails to create copy function that successfully copies portmap file to Container",
			mkTmpDir:        mockData{data: "tmpdir", times: 1},
			removeAll:       mockData{times: 1},
			fprintln:        mockData{data: 0, times: 1},
			inspect:         mockData{data: inspectData, times: 1},
			copyFromArchive: mockData{data: func() error { return errors.New("fail") }, times: 1},
			create:          mockData{data: fileInfo, times: 1},
			get:             mockData{err: nil, times: 1},
			expected:        "error copying portmap file from host to Container: 1 error occurred:\n\t* error copying portmap file to Container: fail",
		},
		{
			name:            "Successfully copies file from host to Container",
			mkTmpDir:        mockData{data: "tmpdir", times: 1},
			removeAll:       mockData{times: 1},
			fprintln:        mockData{data: 0, times: 1},
			inspect:         mockData{data: inspectData, times: 1},
			copyFromArchive: mockData{data: func() error { return nil }, times: 1},
			create:          mockData{data: fileInfo, times: 1},
			get:             mockData{err: nil, times: 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockFileSystemWrite := mocks.NewMockFileSystemWrite(mockCtrl)
			mockFileSystemWrite.EXPECT().MkdirTemp("", "occ_portmaps").Return(tc.mkTmpDir.data, tc.mkTmpDir.err).Times(tc.mkTmpDir.times)
			mockFileSystemWrite.EXPECT().RemoveAll("tmpdir").Return(tc.removeAll.err).Times(tc.removeAll.times)
			mockFileSystemWrite.EXPECT().Create("tmpdir/portmap").Return(tc.create.data, tc.create.err).Times(tc.create.times)
			mockFileSystemWrite.EXPECT().Fprintln(gomock.Any(), gomock.Any()).Return(tc.fprintln.data, tc.fprintln.err).Times(tc.fprintln.times)

			mockContainer := mocks.NewMockContainer(mockCtrl)
			mockContainer.EXPECT().Inspect(nil, "", nil).Return(tc.inspect.data, tc.inspect.err).Times(tc.inspect.times)
			mockContainer.EXPECT().CopyFromArchive(nil, "", "/tmp", gomock.Any()).Return(tc.copyFromArchive.data, tc.copyFromArchive.err).Times(tc.copyFromArchive.times)

			mockCopier := mocks.NewMockCopier(mockCtrl)
			mockCopier.EXPECT().Get("/", "", buildahCopiah.GetOptions{KeepDirectoryNames: true}, gomock.Any(), gomock.Any()).Return(tc.get.err).Times(tc.get.times)

			err := copyPortmap(mockFileSystemWrite, mockContainer, mockCopier, nil, "")
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
