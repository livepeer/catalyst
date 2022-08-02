package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/livepeer/catalyst/cmd/downloader/constants"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	"github.com/livepeer/catalyst/cmd/downloader/utils"
	glog "github.com/magicsong/color-glog"
)

// GetCommitSHA uses github api to find SHA for the tagged release
func GetCommitSHA(project, tag string) *types.GitRefInfo {
	var refInfo types.GitRefInfo
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/git/ref/tags/%s", project, tag)
	resp, err := http.Get(apiURL)
	if err != nil {
		glog.Fatal(err)
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Fatal(err)
	}
	if err := json.Unmarshal(content, &refInfo); err != nil {
		glog.Fatal(err)
	}
	return &refInfo
}

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
	glog.V(9).Infof("github api response=%d", resp.StatusCode)
	return &tagInfo, nil
}

// GetArtifactVersion fetches correct version for artifact from
// github.
func GetArtifactVersion(release, project string) (string, string) {
	if release == constants.LatestTagReleaseName {
		tagInfo, err := GetLatestRelease(project)
		if err != nil {
			panic(err)
		}
		release = tagInfo.TagName
		glog.V(9).Infof("project=%s, version/tag=%q", project, release)
	}
	refInfo := GetCommitSHA(project, release)
	return release, refInfo.Object.SHA
}

// GenerateArtifactURL wraps a `fmt.Sprintf` to template
func GenerateArtifactURL(project, version, fileName string) string {
	return fmt.Sprintf(constants.TaggedDownloadURLFormat, project, version, fileName)
}

// GetArtifactInfo generates a structure of all necessary information
// from using the Github API
func GetArtifactInfo(platform, architecture, release string, service *types.Service) *types.ArtifactInfo {
	project := service.Strategy.Project
	if len(service.Release) > 0 {
		release = service.Release
	}
	version, commit := GetArtifactVersion(release, project)
	service.Strategy.Commit = commit
	var info = &types.ArtifactInfo{
		Name:         service.Name,
		Platform:     platform,
		Architecture: architecture,
		Version:      version,
	}
	extension := utils.PlatformExt(platform)
	packageName := fmt.Sprintf("livepeer-%s", service.Name)
	if len(service.Binary) > 0 {
		packageName = service.Binary
	}
	info.ArchiveFileName = fmt.Sprintf("%s-%s-%s.%s", packageName, info.Platform, info.Architecture, extension)
	if service.SrcFilenames != nil {
		packageName = service.Name
		platArch := fmt.Sprintf("%s-%s", platform, architecture)
		name, ok := service.SrcFilenames[platArch]
		if !ok {
			panic(fmt.Errorf("%s build not found in srcFilenames for %s", service.Name, platArch))
		}
		info.ArchiveFileName = name
	}
	info.Binary = packageName
	info.ArchiveURL = GenerateArtifactURL(project, info.Version, info.ArchiveFileName)

	if !service.SkipChecksum {
		info.ChecksumFileName = fmt.Sprintf("%s_%s", info.Version, constants.ChecksumFileSuffix)
		info.ChecksumURL = GenerateArtifactURL(project, info.Version, info.ChecksumFileName)
	}

	if !service.SkipGPG {
		info.SignatureFileName = fmt.Sprintf("%s.%s", info.ArchiveFileName, constants.SignatureFileExtension)
		info.SignatureURL = GenerateArtifactURL(project, info.Version, info.SignatureFileName)
	}
	return info
}
