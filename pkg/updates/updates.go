package updates

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

// Version is the defined version (from git tag) of the binary. This is added
// as part of the build/release process
var Version string

// BuildCommit is the defined sha from git of the binary from when it was built
// this will only be added as part of the build/release process
var BuildCommit string

type UpdateConfig struct {
	GithubReleaseEndpoint string
}

type ReleaseResponse struct {
	Url string `json:"html_url"`
	Tag string `json:"tag_name"`
}

type UpdateResponse struct {
	CurrentVersion string
	LatestVersion  string
	UpdateUrl      string
}

func CheckForUpdates(updateConfig *UpdateConfig) (*UpdateResponse, error) {
	updateResp := &UpdateResponse{CurrentVersion: Version}

	resp, err := http.Get(updateConfig.GithubReleaseEndpoint)
	if err != nil {
		log.Warn("Error contacting Github API to check for new releases.")
		return updateResp, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Warn("Error parsing Github API response while checking for new releases.")
		return updateResp, err
	}

	releaseResp := &ReleaseResponse{}
	err = json.Unmarshal(body, releaseResp)
	if err != nil {
		log.Warn("Error unmarshaling Github API response while checking for new releases.")
		return updateResp, err
	}

	updateResp.LatestVersion = releaseResp.Tag
	updateResp.UpdateUrl = releaseResp.Url
	return updateResp, nil
}

func (u *UpdateResponse) HasAvailableUpdate(v string) bool {
	if semver.Compare(u.LatestVersion, u.CurrentVersion) > 1 {
		return true
	}
	return false
}
