package github

import "testing"

func TestTagInformation(t *testing.T) {
	projects := []string{
		"livepeer/stream-tester",
		"livepeer/livepeer-data",
		"livepeer/transcode-cli",
		"livepeer/livepeer-com",
		"livepeer/go-livepeer",
	}
	for _, project := range projects {
		GetLatestRelease(project)
	}
}
