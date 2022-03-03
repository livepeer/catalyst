package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/livepeer/livepeer-in-a-box/internal/constants"
	"github.com/livepeer/livepeer-in-a-box/internal/types"
	"github.com/livepeer/livepeer-in-a-box/internal/utils"
)

// GetLatestRelease uses github API to identify information about
// latest tag for a project.
func GetLatestRelease(project string) (*types.TagInformation, error) {
	glog.Infof("Fetching tag information for %s", project)
	var tagInfo types.TagInformation
	var apiURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", project)
	resp, err := http.Get(apiURL)
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

// GetArtifactVersion fetches correct version for artifact from
// github.
func GetArtifactVersion(release, project string) string {
	if release == constants.LatestTagReleaseName {
		tagInfo, err := GetLatestRelease(project)
		if err != nil {
			panic(err)
		}
		release = tagInfo.TagName
	}
	return release
}

// GenerateArtifactURL wraps a `fmt.Sprintf` to template
func GenerateArtifactURL(project, version, fileName string) string {
	return fmt.Sprintf(constants.TaggedDownloadURLFormat, project, version, fileName)
}

// GetArtifactInfo generates a structure of all necessary information
// from using the Github API
func GetArtifactInfo(platform, architecture, release string, service types.Service) *types.ArtifactInfo {
	if len(service.Release) > 0 {
		release = service.Release
	}
	extension := utils.PlatformExt(platform)
	packageName := fmt.Sprintf("livepeer-%s", service.Name)
	if len(service.Binary) > 0 {
		packageName = service.Binary
	}
	var info = &types.ArtifactInfo{
		Name:         service.Name,
		Platform:     platform,
		Architecture: architecture,
		Binary:       packageName,
		Version:      GetArtifactVersion(release, service.Project),
	}
	info.ArchiveFileName = fmt.Sprintf("%s-%s-%s.%s", info.Binary, info.Platform, info.Architecture, extension)
	info.SignatureFileName = fmt.Sprintf("%s.%s", info.ArchiveFileName, constants.SignatureFileExtension)
	info.ChecksumFileName = fmt.Sprintf("%s_%s", info.Version, constants.ChecksumFileSuffix)
	info.ArchiveURL = GenerateArtifactURL(service.Project, info.Version, info.ArchiveFileName)
	info.SignatureURL = GenerateArtifactURL(service.Project, info.Version, info.SignatureFileName)
	info.ChecksumURL = GenerateArtifactURL(service.Project, info.Version, info.ChecksumFileName)
	return info
}
