package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

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
func DownloadService(flags types.CliFlags, manifest *types.BoxManifest, service *types.Service) error {
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
		glog.Fatal("couldn't get project information!")
	}
	glog.Infof("will download %s to %q", projectInfo.Name, downloadPath)
	glog.V(5).Infof("name=%s release=%s commit=%s", projectInfo.Name, service.Release, service.Strategy.Commit)

	// Download archive
	archivePath := filepath.Join(downloadPath, projectInfo.ArchiveFileName)
	err := utils.DownloadFile(archivePath, projectInfo.ArchiveURL, flags.SkipDownloaded)
	if err != nil {
		return err
	}

	// Download signature
	if !service.SkipGPG {
		glog.V(3).Infof("verifying GPG signature for service=%s archive=%s file=%s", service.Name, archivePath, projectInfo.SignatureFileName)
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
		glog.V(3).Infof("verifying SHA checksum for service=%s file=%s", service.Name, projectInfo.ChecksumFileName)
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

	glog.Infof("downloaded %s. Getting ready for extraction!", projectInfo.ArchiveFileName)
	if projectInfo.Platform == "windows" {
		glog.V(7).Info("extracting zip archive!")
		err = ExtractZipArchive(archivePath, downloadPath, service)
		if err != nil {
			return err
		}
	} else {
		glog.V(7).Infof("extracting tarball archive!")
		err = ExtractTarGzipArchive(archivePath, downloadPath, service)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExtractZipArchive processes a zip file and extracts a single file
// from the service definition.
func ExtractZipArchive(archiveFile, extractPath string, service *types.Service) error {
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
			glog.V(9).Infof("extracting to %q", outputPath)
			outfile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			reader, _ := file.Open()
			if _, err := io.Copy(outfile, reader); err != nil {
				glog.Error("failed to create file")
			}
			outfile.Chmod(fs.FileMode(file.Mode()))
			outfile.Close()
		}
	}
	return nil
}

// ExtractTarGzipArchive processes a tarball file and extracts a
// single file from the service definition.
func ExtractTarGzipArchive(archiveFile, extractPath string, service *types.Service) error {
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
		if strings.HasSuffix(header.Name, "/") {
			glog.V(9).Infof("skpping directory %s", header.Name)
			continue
		}
		if strings.HasSuffix(header.Name, service.ArchivePath) {
			if output == "" {
				output = filepath.Join(extractPath, path.Base(header.Name))
			}
			glog.V(9).Infof("extracting to %q", output)
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
		if os.IsNotExist(err) {
			glog.Infof("No manifest detected at %s, downloader continuing", cliFlags.ManifestFile)
			ExecNext(cliFlags)
			return
		}
		glog.Fatal(err)
		return
	}

	for _, element := range manifest.Box {
		if element.Skip {
			continue
		}
		waitGroup.Add(1)
		go func(element *types.Service) {
			glog.V(8).Infof("triggering async task for %s", element.Name)
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
			glog.V(9).Infof("Cleaning up %s", fullpath)
			err = os.Remove(fullpath)
			if err != nil {
				glog.Fatal(err)
			}
		}
	}
	ExecNext(cliFlags)
}

// Done! Move on to the provided next application, if it exists.
func ExecNext(cliFlags types.CliFlags) {
	if len(cliFlags.ExecCommand) == 0 {
		// Nothing to do.
		return
	}
	glog.Infof("downloader complete, now we will exec %v", cliFlags.ExecCommand)
	execErr := syscall.Exec(cliFlags.ExecCommand[0], cliFlags.ExecCommand, os.Environ())
	if execErr != nil {
		glog.Fatalf("error running next command: %s", execErr)
	}
}

func main() {
	Run(types.BuildFlags{Version: Version})
}
