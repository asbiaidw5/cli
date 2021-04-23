// Package update is checking for a new version of Pscale and informs the user
// to update. Most of the logic is copied from cli/cli:
// https://github.com/cli/cli/blob/trunk/internal/update/update.go and updated
// to our own needs.
package update

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cli/safeexec"
	"github.com/fatih/color"
	"github.com/hashicorp/go-version"
	"github.com/planetscale/cli/internal/config"
	"gopkg.in/yaml.v2"
)

type UpdateInfo struct {
	Update      bool
	Reason      string
	ReleaseInfo *ReleaseInfo
}

// ReleaseInfo stores information about a release
type ReleaseInfo struct {
	Version     string    `json:"tag_name"`
	URL         string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// StateEntry stores the information we have checked for a new version. It's
// used to decide whether to check for a new version or not.
type StateEntry struct {
	CheckedForUpdateAt time.Time   `yaml:"checked_for_update_at"`
	LatestRelease      ReleaseInfo `yaml:"latest_release"`
}

// CheckVersion checks for the given build version whether there is a new
// version of the CLI or not.
func CheckVersion(buildVersion string) error {
	path, err := stateFilePath()
	if err != nil {
		return err
	}

	updateInfo, err := checkVersion(
		buildVersion,
		path,
		latestVersion,
	)
	if err != nil {
		return fmt.Errorf("skipping update, error: %s", err)
	}

	if !updateInfo.Update {
		return fmt.Errorf("skipping update, reason: %s", updateInfo.Reason)
	}

	fmt.Fprintf(os.Stderr, "\n\n%s %s → %s\n",
		color.BlueString("A new release of pscale is available:"),
		color.CyanString(buildVersion),
		color.CyanString(updateInfo.ReleaseInfo.Version))

	if isUnderHomebrew() {
		fmt.Fprintf(os.Stderr, "To upgrade, run: %s\n", "brew update && brew upgrade pscale")
	}
	fmt.Fprintf(os.Stderr, "%s\n", color.YellowString(updateInfo.ReleaseInfo.URL))
	return nil
}

func checkVersion(buildVersion, path string, latestVersionFn func(addr string) (*ReleaseInfo, error)) (*UpdateInfo, error) {
	if os.Getenv("PSCALE_NO_UPDATE_NOTIFIER") != "" {
		return &UpdateInfo{
			Update: false,
			Reason: "PSCALE_NO_UPDATE_NOTIFIER is set",
		}, nil
	}

	stateEntry, _ := getStateEntry(path)
	if stateEntry != nil && time.Since(stateEntry.CheckedForUpdateAt).Hours() < 24 {
		return &UpdateInfo{
			Update: false,
			Reason: "Latest version was already checked",
		}, nil
	}

	addr := "https://api.github.com/repos/planetscale/cli/releases/latest"
	info, err := latestVersionFn(addr)
	if err != nil {
		return nil, err
	}

	err = setStateEntry(path, time.Now(), *info)
	if err != nil {
		return nil, err
	}

	v1, err := version.NewVersion(info.Version)
	if err != nil {
		return nil, err
	}

	v2, err := version.NewVersion(buildVersion)
	if err != nil {
		return nil, err
	}

	if v1.LessThanOrEqual(v2) {
		return &UpdateInfo{
			Update: false,
			Reason: fmt.Sprintf("Latest version (%s) is less than or equal to current build version (%s)",
				info.Version, buildVersion),
			ReleaseInfo: info,
		}, nil
	}

	return &UpdateInfo{
		Update: true,
		Reason: fmt.Sprintf("Latest version (%s) is greater than the current build version (%s)",
			info.Version, buildVersion),
		ReleaseInfo: info,
	}, nil

}

func latestVersion(addr string) (*ReleaseInfo, error) {
	req, err := http.NewRequest("GET", addr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	getToken := func() string {
		if t := os.Getenv("GH_TOKEN"); t != "" {
			return t
		}
		return os.Getenv("GITHUB_TOKEN")
	}

	if token := getToken(); token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}

	client := &http.Client{Timeout: time.Second * 15}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !success {
		return nil, fmt.Errorf("error fetching latest release: %v", string(out))
	}

	var info *ReleaseInfo
	err = json.Unmarshal(out, &info)
	if err != nil {
		return nil, err
	}

	return info, nil
}

// copied from: https://github.com/cli/cli/blob/trunk/cmd/gh/main.go#L298
func isUnderHomebrew() bool {
	binary := "pscale"
	if exe, err := os.Executable(); err == nil {
		binary = exe
	}

	brewExe, err := safeexec.LookPath("brew")
	if err != nil {
		return false
	}

	brewPrefixBytes, err := exec.Command(brewExe, "--prefix").Output()
	if err != nil {
		return false
	}

	brewBinPrefix := filepath.Join(strings.TrimSpace(string(brewPrefixBytes)), "bin") + string(filepath.Separator)
	return strings.HasPrefix(binary, brewBinPrefix)
}

func getStateEntry(stateFilePath string) (*StateEntry, error) {
	content, err := ioutil.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}

	var stateEntry StateEntry
	err = yaml.Unmarshal(content, &stateEntry)
	if err != nil {
		return nil, err
	}

	return &stateEntry, nil
}

func setStateEntry(stateFilePath string, t time.Time, r ReleaseInfo) error {
	data := StateEntry{
		CheckedForUpdateAt: t,
		LatestRelease:      r,
	}

	content, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	_ = ioutil.WriteFile(stateFilePath, content, 0600)

	return nil
}

func stateFilePath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}

	return path.Join(dir, "state.yml"), nil
}
