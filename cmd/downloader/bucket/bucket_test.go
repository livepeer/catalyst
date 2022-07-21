package bucket

import (
	"runtime"
	"testing"

	"github.com/livepeer/catalyst/cmd/downloader/constants"
	"github.com/livepeer/catalyst/cmd/downloader/types"
)

// func TestTagInformation(t *testing.T) {
// 	projects := []string{
// 		"livepeer/stream-tester",
// 	}
// 	for _, project := range projects {
// 		_, err := GetLatestRelease(project)
// 		if err != nil {
// 			t.Error("could not fetch tag information")
// 			t.Fail()
// 		}
// 	}
// }

func TestArtifactInfo(t *testing.T) {
	serviceInfo := types.Service{
		Name:    "mistserver",
		Release: "eli/json-manifest",
		Strategy: struct {
			Download string `yaml:"download"`
			Project  string `yaml:"project"`
		}{
			Download: "bucket",
			Project:  "mistserver",
		},
	}
	info := GetArtifactInfo(runtime.GOOS, runtime.GOARCH, constants.LatestTagReleaseName, serviceInfo)
	if info.Version == "" {
		t.Error("The binary name generated for service doesn't match")
		t.Fail()
	}
}
