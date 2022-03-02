package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/livepeer/livepeer-in-a-box/internal/constants"
	"github.com/livepeer/livepeer-in-a-box/internal/types"
)

func GetLatestRelease(project string) (*types.TagInformation, error) {
	var tagInfo types.TagInformation
	var apiUrl = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", project)
	resp, err := http.Get(apiUrl)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	if err := json.Unmarshal(content, &tagInfo); err != nil {
		glog.Error(err)
		return nil, err
	}
	return &tagInfo, nil
}

func GetArtifactVersion(release, project string) string {
	if release == constants.LATEST_TAG_RELEASE_NAME {
		tagInfo, err := GetLatestRelease(project)
		if err != nil {
			panic(err)
		}
		release = tagInfo.TagName
	}
	return release
}

func GenerateArtifactURL(platform, release, architecture, extension string, serviceElement types.Service) (string, string) {
	if len(serviceElement.Release) > 0 {
		release = serviceElement.Release
	}
	release = GetArtifactVersion(release, serviceElement.Project)
	packageName := fmt.Sprintf("livepeer-%s", serviceElement.Name)
	if len(serviceElement.Binary) > 0 {
		packageName = serviceElement.Binary
	}
	archiveName := fmt.Sprintf("%s-%s-%s.%s", packageName, platform, architecture, extension)
	urlFormat := constants.TAGGED_DOWNLOAD_URL_FORMAT
	return fmt.Sprintf(urlFormat, serviceElement.Project, release, archiveName), archiveName
}

func GetArtifactInfo(platform, architecture string, service types.Service) *types.ArtifactInfo {
	var info = &types.ArtifactInfo{}
	return info
}
