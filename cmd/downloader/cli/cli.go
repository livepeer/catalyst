package cli

import (
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

func ValidateFlags(flags *types.CliFlags) error {
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

	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet(constants.AppName, flag.ExitOnError)

	AddDownloaderFlags(fs, &cliFlags)

	version := fs.Bool("version", false, "Get version information")

	if *version {
		fmt.Printf("catalyst version: %s\n", buildFlags.Version)
		os.Exit(0)
	}

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("CATALYST_DOWNLOADER"),
		ff.WithEnvVarSplit(","),
	)
	flag.CommandLine.Parse(nil)
	vFlag.Value.Set(cliFlags.Verbosity)

	err := ValidateFlags(&cliFlags)
	if err != nil {
		glog.Fatal(err)
	}
	return cliFlags, err
}

// Populate a provided flagset with downloader flags
func AddDownloaderFlags(fs *flag.FlagSet, cliFlags *types.CliFlags) {
	goos := runtime.GOOS
	if os.Getenv("GOOS") != "" {
		goos = os.Getenv("GOOS")
	}

	goarch := runtime.GOARCH
	if os.Getenv("GOARCH") != "" {
		goarch = os.Getenv("GOARCH")
	}

	fs.StringVar(&cliFlags.Verbosity, "v", "3", "Log verbosity. Integer value from 0 to 9")
	fs.StringVar(&cliFlags.Platform, "platform", goos, "One of linux/windows/darwin")
	fs.StringVar(&cliFlags.Architecture, "architecture", goarch, "System architecture (amd64/arm64)")
	fs.StringVar(&cliFlags.DownloadPath, "path", fmt.Sprintf(".%sbin", string(os.PathSeparator)), "Path to store binaries")
	fs.StringVar(&cliFlags.ManifestFile, "manifest", "manifest.yaml", "Path (or URL) to manifest yaml file")
	fs.BoolVar(&cliFlags.SkipDownloaded, "skip-downloaded", false, "Skip already downloaded archive (if found)")
	fs.BoolVar(&cliFlags.Cleanup, "cleanup", true, "Cleanup downloaded archives after extraction")
	fs.BoolVar(&cliFlags.UpdateManifest, "update-manifest", false, "Update the manifest file commit shas from releases prior to downloading")
	fs.BoolVar(&cliFlags.Download, "download", true, "Actually do a download. Only useful for -update-manifest=true -download=false")
}
