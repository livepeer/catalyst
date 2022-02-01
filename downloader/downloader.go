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

	"github.com/peterbourgon/ff/v3"
	"gopkg.in/yaml.v2"
)

type BuildFlags struct {
	Version string
}

type CliFlags struct {
	Version      string
	DownloadOnly bool
	DownloadPath string
	Platform     string
	Architecture string
	ManifestFile string
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
	platformExtMap := map[string]string{
		"linux":   "tar.gz",
		"darwin":  "tar.gz",
		"windows": "zip",
	}
	return platformExtMap[platform]
}

func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

func DownloadFile(path, url string) error {
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return nil
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
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
	downloadPath := filepath.Join(flags.DownloadPath, archiveName)
	DownloadFile(downloadPath, archiveUrl)
	if archiveExt == "zip" {
		ExtractZipArchive(downloadPath, flags.DownloadPath, service.ArchivePath)
	} else {
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
	urlFormat := "%s/releases/download/%s/%s"
	if release == "latest" {
		urlFormat = "%s/releases/%s/download/%s"
	}
	return fmt.Sprintf(urlFormat, serviceElement.Src, release, archiveName), archiveName
}

func ParseYamlManifest(manifestPath string) BoxManifest {
	var manifestConfig BoxManifest
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
		return errors.New(fmt.Sprintf(
			"Invalid combination of platform+architecture detected: %s+%s",
			flags.Platform,
			flags.Architecture,
		))
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
			outfile, err := os.Create(path)
			CheckError(err)
			reader, _ := file.Open()
			if _, err := io.Copy(outfile, reader); err != nil {
				fmt.Println("Failed to create file")
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
			outfile, err := os.Create(path)
			CheckError(err)
			if _, err := io.Copy(outfile, tarReader); err != nil {
				fmt.Println("Failed to create file")
			}
			outfile.Chmod(fs.FileMode(header.Mode))
			outfile.Close()
		}
	}
}

func Run(buildFlags BuildFlags) {
	cliFlags := CliFlags{}
	fs := flag.NewFlagSet("box-livepeer", flag.ExitOnError)

	fs.StringVar(&cliFlags.Platform, "platform", runtime.GOOS, "One of linux/windows/darwin")
	fs.StringVar(&cliFlags.Architecture, "architecture", runtime.GOARCH, "System architecture (amd64/arm64)")
	fs.StringVar(&cliFlags.DownloadPath, "path", fmt.Sprintf(".%sbin", string(os.PathSeparator)), "Path to store binaries")
	fs.StringVar(&cliFlags.ManifestFile, "manifest", "manifest.yaml", "Path to manifest yaml file")

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("LP"),
		ff.WithEnvVarSplit(","),
	)

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
}
