package main

import (
	"github.com/livepeer/catalyst/cmd/downloader/cli"
	"github.com/livepeer/catalyst/cmd/downloader/downloader"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	glog "github.com/magicsong/color-glog"
)

var Version = "undefined"

func main() {
	cliFlags, err := cli.GetCliFlags(types.BuildFlags{Version: Version})
	if err != nil {
		glog.Fatalf("error parsing cli flags: %s", err)
		return
	}
	err = downloader.Run(cliFlags)
	if err != nil {
		glog.Fatalf("error running downloader: %s", err)
	}
}
