package main

import (
	"github.com/livepeer/catalyst/cmd/downloader"
	"github.com/livepeer/catalyst/cmd/downloader/types"
)

var Version = "undefined"

func main() {
	downloader.Run(types.BuildFlags{Version: Version})
}
