package main

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"

	"github.com/icza/dyno"
	"github.com/livepeer/catalyst/cmd/downloader/cli"
	"github.com/livepeer/catalyst/cmd/downloader/downloader"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	glog "github.com/magicsong/color-glog"
	"gopkg.in/yaml.v3"
)

var Version = "undefined"

// currently this is extremely similar to the entrypoint at ../downloader/downloader.go
// but that one will stay just a downloader and this binary may gain other functionality

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
	execNext(cliFlags)
}

// Done! Move on to the provided next application, if it exists.
func execNext(cliFlags types.CliFlags) {
	if len(cliFlags.ExecCommand) == 0 {
		// Nothing to do.
		return
	}
	configStr, err := handleConfigFile("/home/iameli/code/catalyst/config/full-stack.yaml")
	if err != nil {
		panic(err)
	}
	panic(configStr)
	glog.Infof("downloader complete, now we will exec %v", cliFlags.ExecCommand)
	execErr := syscall.Exec(cliFlags.ExecCommand[0], cliFlags.ExecCommand, os.Environ())
	if execErr != nil {
		glog.Fatalf("error running next command: %s", execErr)
	}
}
