package utils

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/livepeer/catalyst/cmd/downloader/constants"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	glog "github.com/magicsong/color-glog"
	"gopkg.in/yaml.v3"
)

func IsSupportedPlatformArch(platform, arch string) bool {
	glog.Infof("Checking if we support platform=%q and arch=%q", platform, arch)
	switch platform {
	case "linux",
		"darwin":
		switch arch {
		case "arm64",
			"amd64":
			return true
		}
		break
	case "windows":
		return arch == "amd64"
	}
	return false
}

func ParseYamlManifest(manifestPath string, isURL bool) (*types.BoxManifest, error) {
	var manifestConfig types.BoxManifest
	var file []byte
	glog.Infof("Reading manifest file=%q", manifestPath)
	glog.V(9).Infof("manifestPath=%s isURL=%t", manifestPath, isURL)
	if !isURL {
		file, _ = ioutil.ReadFile(manifestPath)
	} else {
		response, err := http.Get(manifestPath)
		if err != nil || response.StatusCode != 200 {
			return nil, err
		}
		glog.V(9).Infof("response=%v", response)
		file, err = ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
	}
	err := yaml.Unmarshal(file, &manifestConfig)
	if err != nil {
		return nil, err
	}
	if manifestConfig.Version != "3.0" {
		return nil, fmt.Errorf("invalid manifest version %q. Currently supported versions: 3.0", manifestConfig.Version)
	}
	return &manifestConfig, nil
}

func IsFileExists(path string) bool {
	glog.V(6).Infof("Checking if file exists at path=%q", path)
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

func CleanBranchName(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}

func PlatformExt(platform string) string {
	glog.Infof("Fetching archive extension for %q systems.", platform)
	platformExtMap := map[string]string{
		"linux":   constants.TarFileExtension,
		"darwin":  constants.TarFileExtension,
		"windows": constants.ZipFileExtension,
	}
	return platformExtMap[platform]
}

func IsCleanupFile(name string) bool {
	return strings.HasSuffix(name, constants.ZipFileExtension) || strings.HasSuffix(name, constants.TarFileExtension) || strings.HasSuffix(name, ".sig") || strings.HasSuffix(name, "_checksums.txt")
}

func DownloadFile(path, url string, skipDownloaded bool) error {
	glog.Infof("Downloading %s", url)
	if skipDownloaded && IsFileExists(path) {
		glog.Infof("File already downloaded. Skipping!")
		return nil
	}
	url = url + "?cachebust=true"
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	glog.Infof("Response statusCode=%d", resp.StatusCode)
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d while downloading %q", resp.StatusCode, url)
	}
	defer resp.Body.Close()
	tempPath := fmt.Sprintf("%s.TEMP", path)
	out, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	err = os.Rename(tempPath, path)
	return err
}
