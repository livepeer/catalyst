package cli

import (
	"flag"
	"fmt"
	"os"

	downloaderCli "github.com/livepeer/catalyst/cmd/downloader/cli"
	"github.com/livepeer/catalyst/cmd/downloader/constants"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	"github.com/peterbourgon/ff/v3"
)

// GetCliFlags reads command-line arguments and generates a struct
// with useful values set after parsing the same.
func GetCliFlags(buildFlags types.BuildFlags) (types.CliFlags, error) {
	cliFlags := types.CliFlags{}
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet(constants.AppName, flag.ExitOnError)

	downloaderCli.AddDownloaderFlags(fs, &cliFlags)

	fs.StringVar(&cliFlags.MistController, "mist-controller", "MistController", "Path to MistController binary to exec when done")
	fs.BoolVar(&cliFlags.Exec, "exec", true, "Exec MistController when (optional) update is complete")
	fs.StringVar(&cliFlags.ConfigStack, "config", "/etc/livepeer/catalyst.yaml", "Path to multiple Catalyst config files to use. Can contain multiple entries e.g. /conf1:/conf2")

	version := fs.Bool("version", false, "Get version information")

	if *version {
		fmt.Printf("catalyst version: %s\n", buildFlags.Version)
		os.Exit(0)
	}

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("CATALYST"),
		ff.WithEnvVarSplit(","),
	)
	flag.CommandLine.Parse(nil)
	vFlag.Value.Set(cliFlags.Verbosity)

	err := downloaderCli.ValidateFlags(&cliFlags)
	if err != nil {
		return cliFlags, err
	}
	return cliFlags, err
}
