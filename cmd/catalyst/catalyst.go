package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/livepeer/catalyst/cmd/catalyst/cli"
	"github.com/livepeer/catalyst/cmd/catalyst/config"
	"github.com/livepeer/catalyst/cmd/downloader/downloader"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	glog "github.com/magicsong/color-glog"
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
	if cliFlags.Download {
		err = downloader.Run(cliFlags)
		if err != nil {
			glog.Fatalf("error running downloader: %s", err)
		}
	}
	err = execNext(cliFlags)
	if err != nil {
		glog.Fatalf("error executing MistController: %s", err)
	}
}

// Done! Move on to the provided next application, if it exists.
func execNext(cliFlags types.CliFlags) error {
	jsonBytes, err := config.HandleConfigStack(cliFlags.ConfigStack)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "catalyst-generated-*.json")
	if err != nil {
		return err
	}
	_, err = f.Write(jsonBytes)
	if err != nil {
		return err
	}
	glog.Infof("downloader complete, now we will exec %v", cliFlags.MistController)
	binary, err := exec.LookPath(cliFlags.MistController)
	if err != nil {
		return err
	}
	args := []string{binary, "-c", f.Name()}
	execErr := syscall.Exec(binary, args, os.Environ())
	if execErr != nil {
		glog.Fatalf("error running next command: %s", execErr)
	}
	return nil
}
