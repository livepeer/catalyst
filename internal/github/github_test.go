package github

import (
	"runtime"
	"testing"

	"github.com/livepeer/livepeer-in-a-box/internal/types"
)

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
	serviceInfo := types.Service{
		Name:    "api",
		Project: "livepeer/livepeer-com",
	}
	info := GetArtifactInfo(runtime.GOOS, runtime.GOARCH, "latest", serviceInfo)
	if info.Binary != "livepeer-api" {
		t.Error("The binary name generated for service doesn't match")
		t.Fail()
	}
}
