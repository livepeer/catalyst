package bucket

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

// GetArtifactVersion fetches correct version for artifact from
// google cloud bucket.
func GetArtifactVersion(buildInfo types.BuildManifestInformation) string {
	return buildInfo.Commit
}

// GetBuildInformation pulls in build manifest from bucket.
func GetBuildInformation(release, project string) (*types.BuildManifestInformation, error) {
	var buildInfo *types.BuildManifestInformation
	resp, err := http.Get(fmt.Sprintf(constants.BucketManifestURLFormat, project, release))
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	if err := json.Unmarshal(content, &buildInfo); err != nil {
		glog.Error(err)
		return nil, err
	}
	return buildInfo, nil
}

// GenerateArtifactURL wraps a `fmt.Sprintf` to template
func GenerateArtifactURL(project, version, fileName string) string {
	return fmt.Sprintf(constants.BucketDownloadURLFormat, project, version, fileName)
}

// GetArtifactInfo generates a structure of all necessary information
// from the Google Cloud Storage bucket
func GetArtifactInfo(platform, architecture, release string, service *types.Service) *types.ArtifactInfo {
	if len(service.Release) == 0 {
		glog.Fatalf("Bucket type strategy requires a branch name as `release` value. Found %s at root", release)
		panic("")
	}

	project := service.Strategy.Project
	release = utils.CleanBranchName(service.Release)
	buildInfo, err := GetBuildInformation(release, project)
	if err != nil {
		glog.Fatal(err)
	}
	service.Strategy.Commit = buildInfo.Commit

	var info = &types.ArtifactInfo{
		Name:         service.Name,
		Platform:     platform,
		Architecture: architecture,
		Version:      GetArtifactVersion(*buildInfo),
	}

	extension := utils.PlatformExt(platform)
	packageName := fmt.Sprintf("livepeer-%s", service.Name)
	if len(service.Binary) > 0 {
		packageName = service.Binary
	}
	info.ArchiveFileName = fmt.Sprintf("%s-%s-%s.%s", packageName, info.Platform, info.Architecture, extension)
	if buildInfo.SrcFilenames != nil && service.SrcFilenames == nil {
		service.SrcFilenames = buildInfo.SrcFilenames
	}

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
