package github

import (
	"runtime"
	"testing"

	"github.com/livepeer/catalyst/cmd/downloader/types"
)

func TestCommitSHA(t *testing.T) {
	projects := map[string][]string{
		"livepeer/go-livepeer":   {"v0.5.33", "0b3c88f814dccca70a23022f4366eb8069955955"},
		"livepeer/livepeer-data": {"v0.4.17", "4f6696fb83d15bb738d4ff824d47ad032d8d96f9"},
	}
	for project, tag := range projects {
		refInfo := GetCommitSHA(project, tag[0])
		if refInfo.Object.SHA != tag[1] {
			t.Errorf("invalid commit SHA for project=%s tag=%s", project, tag)
			t.Fail()
		}
	}
}

func TestTagInformation(t *testing.T) {
	projects := []string{
		"livepeer/stream-tester",
		"livepeer/livepeer-data",
		"livepeer/transcode-cli",
		"livepeer/livepeer-com",
		"livepeer/go-livepeer",
	}
	for _, project := range projects {
		_, err := GetLatestRelease(project)
		if err != nil {
			t.Error("could not fetch tag information")
			t.Fail()
		}
	}
}

func TestArtifactInfo(t *testing.T) {
	serviceInfo := &types.Service{
		Name: "api",
		Strategy: &types.DownloadStrategy{
			Project: "livepeer/livepeer-com",
		},
	}
	info := GetArtifactInfo(runtime.GOOS, runtime.GOARCH, "latest", serviceInfo)
	if info.Binary != "livepeer-api" {
		t.Error("The binary name generated for service doesn't match")
		t.Fail()
	}
}
