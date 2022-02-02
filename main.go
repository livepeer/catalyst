package main

import "github.com/livepeer/livepeer-in-a-box/downloader"

var Version = "undefined"

func main() {
	downloader.Run(downloader.BuildFlags{Version: Version})
}
