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
	"github.com/livepeer/livepeer-in-a-box/internal/cli"
	"github.com/livepeer/livepeer-in-a-box/internal/constants"
	"github.com/livepeer/livepeer-in-a-box/internal/types"
	"github.com/livepeer/livepeer-in-a-box/internal/utils"
	"github.com/peterbourgon/ff/v3"
	"gopkg.in/yaml.v2"
)

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

func DownloadService(flags types.CliFlags, manifest types.BoxManifest, service types.Service, wg *sync.WaitGroup) {
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

func GenerateArchiveUrl(platform, release, architecture, extension string, serviceElement types.Service) (string, string) {
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

func ParseYamlManifest(manifestPath string) types.BoxManifest {
	var manifestConfig types.BoxManifest
	glog.Infof("Reading manifest file=%q", manifestPath)
	file, _ := ioutil.ReadFile(manifestPath)
	err := yaml.Unmarshal(file, &manifestConfig)
	utils.CheckError(err)
	if manifestConfig.Version != "2.0" {
		panic(errors.New("Invalid manifest version. Currently supported versions: 2.0"))
	}
	return manifestConfig
}

func ExtractZipArchive(archiveFile, extractPath, archivePath string) {
	if len(archivePath) > 0 && !strings.HasSuffix(archivePath, ".exe") {
		archivePath += ".exe"
	}
	zipReader, err := zip.OpenReader(archiveFile)
	utils.CheckError(err)
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
			utils.CheckError(err)
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
	utils.CheckError(err)
	tarReader := tar.NewReader(archive)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		utils.CheckError(err)
		if strings.HasSuffix(header.Name, archivePath) {
			var path string
			if len(archivePath) > 0 {
				path = filepath.Join(extractPath, archivePath)
			} else {
				path = filepath.Join(extractPath, header.Name)
			}
			glog.Infof("Extracting to %q", path)
			outfile, err := os.Create(path)
			utils.CheckError(err)
			if _, err := io.Copy(outfile, tarReader); err != nil {
				glog.Errorf("Failed to create file: %q", path)
			}
			outfile.Chmod(fs.FileMode(header.Mode))
			outfile.Close()
		}
	}
}

func Run(buildFlags types.BuildFlags) {
	cliFlags := cli.GetCliFlags(buildFlags)
	var waitGroup sync.WaitGroup
	manifest := ParseYamlManifest(cliFlags.ManifestFile)
	for _, element := range manifest.Box {
		if element.Skip {
			continue
		}
		waitGroup.Add(1)
		go DownloadService(cliFlags, manifest, element, &waitGroup)
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
		if strings.HasSuffix(file.Name(), constants.ZIP_FILE_EXTENSION) || strings.HasSuffix(file.Name(), constants.TAR_FILE_EXTENSION) {
			fullpath := filepath.Join(cliFlags.DownloadPath, file.Name())
			glog.V(5).Infof("Cleaning up %s", fullpath)
			err = os.Remove(fullpath)
			if err != nil {
				glog.Fatal(err)
			}
		}
	}
}
