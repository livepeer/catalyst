package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/livepeer/livepeer-in-a-box/internal/constants"
	"github.com/livepeer/livepeer-in-a-box/internal/types"
	"github.com/livepeer/livepeer-in-a-box/internal/utils"
	"github.com/peterbourgon/ff/v3"
)

func validateFlags(flags types.CliFlags) error {
	if !utils.IsSupportedPlatformArch(flags.Platform, flags.Architecture) {
		return fmt.Errorf(
			"Invalid combination of platform+architecture detected: %s+%s",
			flags.Platform,
			flags.Architecture,
		)
	}
	if !utils.IsFileExists(flags.ManifestFile) {
		return errors.New("Invalid path to manifest file!")
	}
	if info, err := os.Stat(flags.DownloadPath); !(err == nil && info.IsDir()) {
		return errors.New("Invalid path provided for downloaded binaries! Check if it exists?")
	}
	return nil
}

// GetCliFlags
func GetCliFlags(buildFlags types.BuildFlags) (types.CliFlags, error) {
	cliFlags := types.CliFlags{}
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet(constants.AppName, flag.ExitOnError)

	fs.StringVar(&cliFlags.Verbosity, "v", "3", "Log verbosity. Integer value from 0 to 9")
	fs.StringVar(&cliFlags.Platform, "platform", runtime.GOOS, "One of linux/windows/darwin")
	fs.StringVar(&cliFlags.Architecture, "architecture", runtime.GOARCH, "System architecture (amd64/arm64)")
	fs.StringVar(&cliFlags.DownloadPath, "path", fmt.Sprintf(".%sbin", string(os.PathSeparator)), "Path to store binaries")
	fs.StringVar(&cliFlags.ManifestFile, "manifest", "manifest.yaml", "Path to manifest yaml file")
	fs.BoolVar(&cliFlags.SkipDownloaded, "skip-downloaded", false, "Skip already downloaded archive (if found)")
	fs.BoolVar(&cliFlags.Cleanup, "cleanup", true, "Cleanup downloaded archives after extraction")

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("LP"),
		ff.WithEnvVarSplit(","),
	)
	flag.CommandLine.Parse(nil)
	vFlag.Value.Set(cliFlags.Verbosity)

	err := validateFlags(cliFlags)
	return cliFlags, err
}
