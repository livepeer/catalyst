package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/livepeer/catalyst/cmd/downloader/bucket"
	"github.com/livepeer/catalyst/cmd/downloader/cli"
	"github.com/livepeer/catalyst/cmd/downloader/github"
	"github.com/livepeer/catalyst/cmd/downloader/types"
	"github.com/livepeer/catalyst/cmd/downloader/utils"
	"github.com/livepeer/catalyst/cmd/downloader/verification"
	glog "github.com/magicsong/color-glog"
)

var Version = "undefined"

// DownloadService works on downloading services for the box to
// machine and extracting the required binaries from artifacts.
func DownloadService(flags types.CliFlags, manifest *types.BoxManifest, service types.Service) error {
	var projectInfo *types.ArtifactInfo
	platform := flags.Platform
	architecture := flags.Architecture
	downloadPath := flags.DownloadPath

	if service.Strategy.Download == "bucket" {
		projectInfo = bucket.GetArtifactInfo(platform, architecture, manifest.Release, service)
	} else {
		projectInfo = github.GetArtifactInfo(platform, architecture, manifest.Release, service)
	}
	if projectInfo == nil {
		glog.Fatal("Couldn't get project information!")
	}
	glog.Infof("Will download %s to %q", projectInfo.Name, downloadPath)

	// Download archive
	archivePath := filepath.Join(downloadPath, projectInfo.ArchiveFileName)
	err := utils.DownloadFile(archivePath, projectInfo.ArchiveURL, flags.SkipDownloaded)
	if err != nil {
		return err
	}

	// Download signature
	if !service.SkipGPG {
		glog.Infof("Doing GPG verification for service=%s", service.Name)
		signaturePath := filepath.Join(downloadPath, projectInfo.SignatureFileName)
		err = utils.DownloadFile(signaturePath, projectInfo.SignatureURL, flags.SkipDownloaded)
		if err != nil {
			return err
		}
		err = verification.VerifyGPGSignature(archivePath, signaturePath)
		if err != nil {
			return err
		}
	}

	// Download checksum
	if !service.SkipChecksum {
		glog.Infof("Doing SHA checksum verification for service=%s", service.Name)
		checksumPath := filepath.Join(downloadPath, projectInfo.ChecksumFileName)
		err = utils.DownloadFile(checksumPath, projectInfo.ChecksumURL, flags.SkipDownloaded)
		if err != nil {
			return err
		}
		err = verification.VerifySHA256Digest(downloadPath, projectInfo.ChecksumFileName)
		if err != nil {
			return err
		}
	}

	glog.Infof("Downloaded %s. Getting ready for extraction!", projectInfo.ArchiveFileName)
	if projectInfo.Platform == "windows" {
		glog.Info("Extracting zip archive!")
		err = ExtractZipArchive(archivePath, downloadPath, service)
		if err != nil {
			return err
		}
	} else {
		glog.Info("Extracting tarball archive!")
		err = ExtractTarGzipArchive(archivePath, downloadPath, service)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExtractZipArchive processes a zip file and extracts a single file
// from the service definition.
func ExtractZipArchive(archiveFile, extractPath string, service types.Service) error {
	var outputPath string
	if len(service.ArchivePath) > 0 && !strings.HasSuffix(service.ArchivePath, ".exe") {
		service.ArchivePath += ".exe"
		outputPath = filepath.Join(extractPath, service.ArchivePath)
	}
	if len(service.OutputPath) > 0 {
		outputPath = filepath.Join(extractPath, service.OutputPath+".exe")
	}
	zipReader, err := zip.OpenReader(archiveFile)
	if err != nil {
		return err
	}
	for _, file := range zipReader.File {
		if strings.HasSuffix(file.Name, service.ArchivePath) {
			if outputPath == "" {
				outputPath = filepath.Join(extractPath, file.Name)
			}
			glog.Infof("Extracting to %q", outputPath)
			outfile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			reader, _ := file.Open()
			if _, err := io.Copy(outfile, reader); err != nil {
				glog.Error("Failed to create file")
			}
			outfile.Chmod(fs.FileMode(file.Mode()))
			outfile.Close()
		}
	}
	return nil
}

// ExtractTarGzipArchive processes a tarball file and extracts a
// single file from the service definition.
func ExtractTarGzipArchive(archiveFile, extractPath string, service types.Service) error {
	var outputPath string
	file, _ := os.Open(archiveFile)
	archive, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(archive)
	if len(service.ArchivePath) > 0 {
		outputPath = filepath.Join(extractPath, service.ArchivePath)
	}
	if len(service.OutputPath) > 0 {
		outputPath = filepath.Join(extractPath, service.OutputPath)
	}
	for {
		header, err := tarReader.Next()
		output := outputPath
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if strings.HasSuffix(header.Name, service.ArchivePath) {
			if output == "" {
				output = filepath.Join(extractPath, header.Name)
			}
			glog.Infof("Extracting to %q", output)
			outfile, err := os.Create(output)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outfile, tarReader); err != nil {
				glog.Errorf("Failed to create file: %q", output)
			}
			if err != nil {
				return err
			}
			outfile.Chmod(fs.FileMode(header.Mode))
			outfile.Close()
		}
	}
	return nil
}

// Run is the entrypoint for main program.
func Run(buildFlags types.BuildFlags) {
	cliFlags, err := cli.GetCliFlags(buildFlags)
	if err != nil {
		glog.Fatal(err)
		return
	}
	var waitGroup sync.WaitGroup
	manifest, err := utils.ParseYamlManifest(cliFlags.ManifestFile, cliFlags.ManifestURL)
	if err != nil {
		glog.Fatal(err)
		return
	}

	for _, element := range manifest.Box {
		if element.Skip {
			continue
		}
		waitGroup.Add(1)
		go func(element types.Service) {
			glog.V(8).Infof("Triggering async task for %s", element.Name)
			err := DownloadService(cliFlags, manifest, element)
			if err != nil {
				glog.Fatalf("failed to download %s: %s", element.Name, err)
			}
			waitGroup.Done()
		}(element)
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
		if utils.IsCleanupFile(file.Name()) {
			fullpath := filepath.Join(cliFlags.DownloadPath, file.Name())
			glog.V(5).Infof("Cleaning up %s", fullpath)
			err = os.Remove(fullpath)
			if err != nil {
				glog.Fatal(err)
			}
		}
	}
}

func main() {
	Run(types.BuildFlags{Version: Version})
}
