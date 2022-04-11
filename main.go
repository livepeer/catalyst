package main

import (
	"github.com/livepeer/livepeer-in-a-box/cmd/downloader"
	"github.com/livepeer/livepeer-in-a-box/internal/types"
)

var Version = "undefined"

func main() {
	downloader.Run(types.BuildFlags{Version: Version})
}
