package main

import (
	"io/ioutil"

	"github.com/livepeer/catalyst/cmd/downloader/bucket"
	"github.com/livepeer/catalyst/cmd/downloader/cli"
	"github.com/livepeer/catalyst/cmd/downloader/constants"
	"github.com/livepeer/catalyst/cmd/downloader/github"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	"github.com/livepeer/catalyst/cmd/downloader/utils"
	glog "github.com/magicsong/color-glog"
	"gopkg.in/yaml.v3"
)

var version = "Unknown"

func GenerateYamlManifest(manifest types.BoxManifest, path string) error {
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path, data, 0644)
	return err
}

func Run(buildFlags types.BuildFlags) {
	var projectInfo *types.ArtifactInfo

	cliFlags, err := cli.GetCliFlags(buildFlags)
	if err != nil {
		glog.Fatal(err)
	}
	manifest, err := utils.ParseYamlManifest(cliFlags.ManifestFile, cliFlags.ManifestURL)
	if err != nil {
		glog.Fatal(err)
	}

	platform := cliFlags.Platform
	architecture := cliFlags.Architecture

	for _, service := range manifest.Box {
		if service.Skip || service.SkipManifestUpdate {
			continue
		}
		if service.Strategy.Download == "" {
			service.Strategy.Download = "github"
		}
		if service.Strategy.Download == "bucket" {
			projectInfo = bucket.GetArtifactInfo(platform, architecture, manifest.Release, service)
		} else if service.Strategy.Download == "github" {
			service.Release = constants.LatestTagReleaseName
			projectInfo = github.GetArtifactInfo(platform, architecture, manifest.Release, service)
			service.Release = projectInfo.Version
		}
		glog.V(8).Infof("gh-version=%q, manifest-version=%q", projectInfo.Version, service.Release)
	}
	err = GenerateYamlManifest(*manifest, cliFlags.ManifestFile)

	if err != nil {
		glog.Fatal(err)
	}
}

func main() {
	Run(types.BuildFlags{Version: version})
}
