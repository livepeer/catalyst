package downloader

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/livepeer/livepeer-in-a-box/internal/cli"
	"github.com/livepeer/livepeer-in-a-box/internal/constants"
	"github.com/livepeer/livepeer-in-a-box/internal/github"
	"github.com/livepeer/livepeer-in-a-box/internal/types"
	"github.com/livepeer/livepeer-in-a-box/internal/utils"
	"github.com/livepeer/livepeer-in-a-box/internal/verification"
	"gopkg.in/yaml.v2"
)

func DownloadService(flags types.CliFlags, manifest types.BoxManifest, service types.Service, wg *sync.WaitGroup) {
	defer wg.Done()
	projectInfo := github.GetArtifactInfo(flags.Platform, flags.Architecture, manifest.Release, service)
	glog.Infof("Will download to %q", flags.DownloadPath)

	b, _ := json.Marshal(projectInfo)
	glog.Info(string(b))
	os.Exit(0)

	// Download archive
	archivePath := filepath.Join(flags.DownloadPath, projectInfo.ArchiveFileName)
	err := utils.DownloadFile(archivePath, projectInfo.ArchiveURL, flags.SkipDownloaded)
	utils.CheckError(err)

	// Download signature
	if !service.SkipGPG {
		signaturePath := filepath.Join(flags.DownloadPath, projectInfo.SignatureFileName)
		err = utils.DownloadFile(signaturePath, projectInfo.SignatureURL, flags.SkipDownloaded)
		utils.CheckError(err)
		err = verification.VerifyGPGSignature(archivePath, signaturePath)
		utils.CheckError(err)
	}

	// Download checksum
	if !service.SkipChecksum {
		checksumPath := filepath.Join(flags.DownloadPath, projectInfo.ChecksumFileName)
		err = utils.DownloadFile(checksumPath, projectInfo.ChecksumURL, flags.SkipDownloaded)
		utils.CheckError(err)
		err = verification.VerifySHA256Digest(flags.DownloadPath, projectInfo.ChecksumFileName)
		utils.CheckError(err)
	}

	glog.Infof("Downloaded %s. Getting ready for extraction!", projectInfo.ArchiveFileName)
	if projectInfo.Platform == "windows" {
		glog.Info("Extracting zip archive!")
		ExtractZipArchive(archivePath, flags.DownloadPath, service)
	} else {
		glog.Info("Extracting tarball archive!")
		ExtractTarGzipArchive(archivePath, flags.DownloadPath, service)
	}
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

func ExtractZipArchive(archiveFile, extractPath string, service types.Service) {
	if len(service.ArchivePath) > 0 && !strings.HasSuffix(service.ArchivePath, ".exe") {
		service.ArchivePath += ".exe"
	}
	zipReader, err := zip.OpenReader(archiveFile)
	utils.CheckError(err)
	for _, file := range zipReader.File {
		if strings.HasSuffix(file.Name, service.ArchivePath) {
			var path string
			if len(service.OutputPath) > 0 {
				path = filepath.Join(extractPath, service.OutputPath+".exe")
			} else if len(service.ArchivePath) > 0 {
				path = filepath.Join(extractPath, service.ArchivePath)
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

func ExtractTarGzipArchive(archiveFile, extractPath string, service types.Service) {
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
		if strings.HasSuffix(header.Name, service.ArchivePath) {
			var path string
			if len(service.OutputPath) > 0 {
				path = filepath.Join(extractPath, service.OutputPath)
			} else if len(service.ArchivePath) > 0 {
				path = filepath.Join(extractPath, service.ArchivePath)
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
		if strings.HasSuffix(file.Name(), constants.ZipFileExtension) || strings.HasSuffix(file.Name(), constants.TarFileExtension) {
			fullpath := filepath.Join(cliFlags.DownloadPath, file.Name())
			glog.V(5).Infof("Cleaning up %s", fullpath)
			err = os.Remove(fullpath)
			if err != nil {
				glog.Fatal(err)
			}
		}
	}
}
