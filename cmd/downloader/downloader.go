package downloader

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/livepeer/livepeer-in-a-box/internal/constants"
	"github.com/peterbourgon/ff/v3"
	"gopkg.in/yaml.v2"
)

type BuildFlags struct {
	Version string
}

type CliFlags struct {
	SkipDownloaded bool
	Cleanup        bool
	DownloadPath   string
	Platform       string
	Architecture   string
	ManifestFile   string
	Verbosity      string
}

type Service struct {
	Name        string `yaml:"name"`
	Src         string `yaml:"src"`
	Binary      string `yaml:"binary,omitempty"`
	Release     string `yaml:"release,omitempty"`
	ArchivePath string `yaml:"archivePath,omitempty"`
	Skip        bool   `yaml:"skip"`
}

type BoxManifest struct {
	Version string    `yaml:"version"`
	Release string    `yaml:"release,omitempty"`
	Box     []Service `yaml:"box,omitempty"`
}

func IsSupportedPlatformArch(platform, arch string) bool {
	glog.Infof("Checking if we support platform: %q and arch: %q", platform, arch)
	switch platform {
	case "linux",
		"darwin":
		switch arch {
		case "arm64",
			"amd64":
			return true
		}
		break
	case "windows":
		return arch == "amd64"
	}
	return false
}

func IsManifestFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func PlatformExt(platform string) string {
	glog.Infof("Fetching archive extension for %q systems.", platform)
	platformExtMap := map[string]string{
		"linux":   "tar.gz",
		"darwin":  "tar.gz",
		"windows": "zip",
	}
	return platformExtMap[platform]
}

func CheckError(err error) {
	if err != nil {
		glog.Fatal(err)
	}
}

func DownloadFile(path, url string, skipDownloaded bool) error {
	glog.V(5).Infof("Downloading %s", url)
	if info, err := os.Stat(path); err == nil && info.Size() > 0 && skipDownloaded {
		glog.Infof("Found already downloaded archive. Skipping!")
		return nil
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	glog.V(9).Infof("Response statusCode=%d", resp.StatusCode)
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d while downloading %s", resp.StatusCode, url)
	}
	defer resp.Body.Close()
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func DownloadService(flags CliFlags, manifest BoxManifest, service Service, wg *sync.WaitGroup) {
	defer wg.Done()
	archiveExt := PlatformExt(flags.Platform)
	archiveUrl, archiveName := GenerateArchiveUrl(flags.Platform, manifest.Release, flags.Architecture, archiveExt, service)
	glog.Infof("Will download %s to %q", archiveName, flags.DownloadPath)
	downloadPath := filepath.Join(flags.DownloadPath, archiveName)
	err := DownloadFile(downloadPath, archiveUrl, flags.SkipDownloaded)
	CheckError(err)
	glog.Infof("Downloaded %s. Getting ready for extraction!", downloadPath)
	if archiveExt == "zip" {
		glog.Info("Extracting zip archive!")
		ExtractZipArchive(downloadPath, flags.DownloadPath, service.ArchivePath)
	} else {
		glog.Info("Extracting tarball archive!")
		ExtractTarGzipArchive(downloadPath, flags.DownloadPath, service.ArchivePath)
	}
}

func GenerateArchiveUrl(platform, release, architecture, extension string, serviceElement Service) (string, string) {
	if len(serviceElement.Release) > 0 {
		release = serviceElement.Release
	}
	packageName := fmt.Sprintf("livepeer-%s", serviceElement.Name)
	if len(serviceElement.Binary) > 0 {
		packageName = serviceElement.Binary
	}
	archiveName := fmt.Sprintf("%s-%s-%s.%s", packageName, platform, architecture, extension)
	urlFormat := constants.TAGGED_DOWNLOAD_URL_FORMAT
	if release == constants.LATEST_TAG_RELEASE_NAME {
		urlFormat = constants.LATEST_DOWNLOAD_URL_FORMAT
	}
	return fmt.Sprintf(urlFormat, serviceElement.Src, release, archiveName), archiveName
}

func ParseYamlManifest(manifestPath string) BoxManifest {
	var manifestConfig BoxManifest
	glog.Infof("Reading mnifest file at %q", manifestPath)
	file, _ := ioutil.ReadFile(manifestPath)
	err := yaml.Unmarshal(file, &manifestConfig)
	CheckError(err)
	if manifestConfig.Version != "1.0" {
		panic(errors.New("Invalid manifest version. Currently supported versions: 1.0"))
	}
	return manifestConfig
}

func ValidateFlags(flags CliFlags) error {
	if !IsSupportedPlatformArch(flags.Platform, flags.Architecture) {
		return fmt.Errorf(
			"Invalid combination of platform+architecture detected: %s+%s",
			flags.Platform,
			flags.Architecture,
		)
	}
	if !IsManifestFileExists(flags.ManifestFile) {
		return errors.New("Invalid path to manifest file!")
	}
	if info, err := os.Stat(flags.DownloadPath); !(err == nil && info.IsDir()) {
		return errors.New("Invalid path provided for downloaded binaries! Check if it exists?")
	}
	return nil
}

func ExtractZipArchive(archiveFile, extractPath, archivePath string) {
	if len(archivePath) > 0 && !strings.HasSuffix(archivePath, ".exe") {
		archivePath += ".exe"
	}
	zipReader, err := zip.OpenReader(archiveFile)
	CheckError(err)
	for _, file := range zipReader.File {
		if strings.HasSuffix(file.Name, archivePath) {
			var path string
			if len(archivePath) > 0 {
				path = filepath.Join(extractPath, archivePath)
			} else {
				path = filepath.Join(extractPath, file.Name)
			}
			glog.Infof("Extracting to %q", path)
			outfile, err := os.Create(path)
			CheckError(err)
			reader, _ := file.Open()
			if _, err := io.Copy(outfile, reader); err != nil {
				glog.Error("Failed to create file")
			}
			outfile.Chmod(fs.FileMode(file.Mode()))
			outfile.Close()
		}
	}
}

func ExtractTarGzipArchive(archiveFile, extractPath, archivePath string) {
	file, _ := os.Open(archiveFile)
	archive, err := gzip.NewReader(file)
	CheckError(err)
	tarReader := tar.NewReader(archive)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		CheckError(err)
		if strings.HasSuffix(header.Name, archivePath) {
			var path string
			if len(archivePath) > 0 {
				path = filepath.Join(extractPath, archivePath)
			} else {
				path = filepath.Join(extractPath, header.Name)
			}
			glog.Infof("Extracting to %q", path)
			outfile, err := os.Create(path)
			CheckError(err)
			if _, err := io.Copy(outfile, tarReader); err != nil {
				glog.Errorf("Failed to create file: %q", path)
			}
			outfile.Chmod(fs.FileMode(header.Mode))
			outfile.Close()
		}
	}
}

func Run(buildFlags BuildFlags) {
	cliFlags := CliFlags{}
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet(constants.APP_NAME, flag.ExitOnError)

	fs.StringVar(&cliFlags.Verbosity, "v", "", "Log verbosity.  {4|5|6}")
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

	err := ValidateFlags(cliFlags)
	CheckError(err)
	var waitGroup sync.WaitGroup
	manifest := ParseYamlManifest(cliFlags.ManifestFile)
	for _, element := range manifest.Box {
		if element.Skip {
			continue
		}
		waitGroup.Add(1)
		DownloadService(cliFlags, manifest, element, &waitGroup)
	}
	waitGroup.Wait()

	if !cliFlags.Cleanup {
		glog.Info("Not cleaning up after extraction")
		return
	}

	files, err := ioutil.ReadDir(cliFlags.DownloadPath)
	if err != nil {
		glog.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".zip") || strings.HasSuffix(file.Name(), ".tar.gz") {
			fullpath := filepath.Join(cliFlags.DownloadPath, file.Name())
			glog.V(5).Infof("Cleaning up %s", fullpath)
			err = os.Remove(fullpath)
			if err != nil {
				glog.Fatal(err)
			}
		}
	}
}
