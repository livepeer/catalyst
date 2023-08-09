package cli

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"runtime"

	"github.com/livepeer/catalyst/cmd/downloader/constants"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	"github.com/livepeer/catalyst/cmd/downloader/utils"
	glog "github.com/magicsong/color-glog"
	"github.com/peterbourgon/ff/v3"
)

func validateFlags(flags *types.CliFlags) error {
	if !utils.IsSupportedPlatformArch(flags.Platform, flags.Architecture) {
		return fmt.Errorf(
			"invalid combination of platform+architecture detected: %s+%s",
			flags.Platform,
			flags.Architecture,
		)
	}
	if !utils.IsFileExists(flags.ManifestFile) {
		manifestURL, err := url.Parse(flags.ManifestFile)
		if err != nil {
			return err
		}
		if manifestURL.Scheme == "https" {
			flags.ManifestURL = true
		} else if len(flags.ExecCommand) == 0 {
			return errors.New("invalid path/url to manifest file")
		}
	}
	if info, err := os.Stat(flags.DownloadPath); !(err == nil && info.IsDir()) {
		err = os.MkdirAll(flags.DownloadPath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetCliFlags reads command-line arguments and generates a struct
// with useful values set after parsing the same.
func GetCliFlags(buildFlags types.BuildFlags) (types.CliFlags, error) {
	cliFlags := types.CliFlags{}
	args := []string{}
	// Handle post-exec string
	for i, arg := range os.Args[1:] {
		if arg == "--" {
			cliFlags.ExecCommand = os.Args[i+2:]
			break
		}
		args = append(args, arg)
	}
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet(constants.AppName, flag.ExitOnError)

	fs.StringVar(&cliFlags.Verbosity, "v", "3", "Log verbosity. Integer value from 0 to 9")
	fs.StringVar(&cliFlags.Platform, "platform", runtime.GOOS, "One of linux/windows/darwin")
	fs.StringVar(&cliFlags.Architecture, "architecture", runtime.GOARCH, "System architecture (amd64/arm64)")
	fs.StringVar(&cliFlags.DownloadPath, "path", fmt.Sprintf(".%sbin", string(os.PathSeparator)), "Path to store binaries")
	fs.StringVar(&cliFlags.ManifestFile, "manifest", "manifest.yaml", "Path (or URL) to manifest yaml file")
	fs.BoolVar(&cliFlags.SkipDownloaded, "skip-downloaded", false, "Skip already downloaded archive (if found)")
	fs.BoolVar(&cliFlags.Cleanup, "cleanup", true, "Cleanup downloaded archives after extraction")
	fs.BoolVar(&cliFlags.UpdateManifest, "update-manifest", false, "Update the manifest file commit shas from releases prior to downloading")
	fs.BoolVar(&cliFlags.Download, "download", true, "Actually do a download. Only useful for -update-manifest=true -download=false")

	version := fs.Bool("version", false, "Get version information")

	if *version {
		fmt.Printf("catalyst version: %s\n", buildFlags.Version)
		os.Exit(0)
	}

	ff.Parse(
		fs, args,
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("CATALYST_DOWNLOADER"),
		ff.WithEnvVarSplit(","),
	)
	flag.CommandLine.Parse(nil)
	vFlag.Value.Set(cliFlags.Verbosity)

	err := validateFlags(&cliFlags)
	if err != nil {
		glog.Fatal(err)
	}
	return cliFlags, err
}
