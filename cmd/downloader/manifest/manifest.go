package manifest

import (
	"bytes"
	"io/ioutil"

	"github.com/livepeer/catalyst/cmd/downloader/bucket"
	"github.com/livepeer/catalyst/cmd/downloader/constants"
	"github.com/livepeer/catalyst/cmd/downloader/github"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	glog "github.com/magicsong/color-glog"
	"gopkg.in/yaml.v3"
)

var version = "Unknown"

func GenerateYamlManifest(manifest types.BoxManifest, path string) error {
	var data bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&data)
	yamlEncoder.SetIndent(2)
	err := yamlEncoder.Encode(&manifest)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path, data.Bytes(), 0644)
	return err
}

// returns a manifest and boolean for whether we successfully wrote one
func UpdateManifest(cliFlags types.CliFlags, m *types.BoxManifest) bool {
	var projectInfo *types.ArtifactInfo

	platform := cliFlags.Platform
	architecture := cliFlags.Architecture

	for _, service := range m.Box {
		if service.Skip || service.SkipManifestUpdate {
			continue
		}
		if service.Strategy.Download == "" {
			service.Strategy.Download = "github"
		}
		if service.Strategy.Download == "bucket" {
			projectInfo = bucket.GetArtifactInfo(platform, architecture, m.Release, service)
		} else if service.Strategy.Download == "github" {
			service.Release = constants.LatestTagReleaseName
			projectInfo = github.GetArtifactInfo(platform, architecture, m.Release, service)
			service.Release = projectInfo.Version
		}
		glog.V(8).Infof("gh-version=%q, manifest-version=%q", projectInfo.Version, service.Release)
	}
	err := GenerateYamlManifest(*m, cliFlags.ManifestFile)

	if err != nil {
		glog.Error(err)
		return false
	}
	return true
}
