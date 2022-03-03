package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/livepeer/livepeer-in-a-box/internal/constants"
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

func IsFileExists(path string) bool {
	glog.Infof("Checking if file exists at path=%q", path)
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
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

func CheckError(err error) {
	if err != nil {
		glog.Fatal(err)
	}
}

func DownloadFile(path, url string, skipDownloaded bool) error {
	glog.V(5).Infof("Downloading %s", url)
	if skipDownloaded && IsFileExists(path) {
		glog.Infof("File already downloaded. Skipping!")
		return nil
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	glog.V(9).Infof("Response statusCode=%d", resp.StatusCode)
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d while downloading %s", resp.StatusCode, url)
	}
	defer resp.Body.Close()
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}
