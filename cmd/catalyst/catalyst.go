package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/golang/glog"
	"github.com/livepeer/catalyst/cmd/downloader/constants"
	"github.com/livepeer/catalyst/config"
	"github.com/peterbourgon/ff/v3"
)

var Version = "undefined"

// currently this is extremely similar to the entrypoint at ../downloader/downloader.go
// but that one will stay just a downloader and this binary may gain other functionality

func main() {
	cli := config.Cli{}
	flag.Set("logtostderr", "true")
	// vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet(constants.AppName, flag.ExitOnError)
	// fs.StringVar(&cli.Verbosity, "v", "3", "Log verbosity. Integer value from 0 to 9")
	fs.StringVar(&cli.PublicURL, "public-url", "http://localhost:8888", "Public-facing URL of your Catalyst node, including protocol and port")
	fs.StringVar(&cli.Secret, "secret", "", "Secret UUID to secure your Catalyst node")
	fs.StringVar(&cli.ConfOutput, "conf-output", "/tmp/catalyst-generated.json", "Secret UUID to secure your Catalyst node")

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("CATALYST"),
		ff.WithEnvVarSplit(","),
	)
	flag.CommandLine.Parse(nil)
	blob, err := config.Config(&cli)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", string(blob))
	err = os.WriteFile(cli.ConfOutput, blob, 0600)
	if err != nil {
		panic(err)
	}
	execNext(cli)
}

// Archiving for when we want to introduce auto-updating:

// func main() {
// 	cliFlags, err := cli.GetCliFlags(types.BuildFlags{Version: Version})
// 	if err != nil {
// 		glog.Fatalf("error parsing cli flags: %s", err)
// 		return
// 	}
// 	err = downloader.Run(cliFlags)
// 	if err != nil {
// 		glog.Fatalf("error running downloader: %s", err)
// 	}
// 	execNext(cliFlags)
// }

// Done! Move on to the provided next application, if it exists.
func execNext(cli config.Cli) {
	fname, err := exec.LookPath("MistController")
	if err != nil {
		glog.Fatalf("error finding MistController: %s", fname)
	}
	glog.Infof("config file written, now we will exec MistController")
	execErr := syscall.Exec(fname, []string{"MistController", "-c", cli.ConfOutput}, os.Environ())
	if execErr != nil {
		glog.Fatalf("error running next command: %s", execErr)
	}
}
