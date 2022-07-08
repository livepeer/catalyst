package downloader

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/livepeer/catalyst/internal/bucket"
	"github.com/livepeer/catalyst/internal/cli"
	"github.com/livepeer/catalyst/internal/github"
	"github.com/livepeer/catalyst/internal/types"
	"github.com/livepeer/catalyst/internal/utils"
	"github.com/livepeer/catalyst/internal/verification"
	"gopkg.in/yaml.v2"
)

// DownloadService works on downloading services for the box to
// machine and extracting the required binaries from artifacts.
func DownloadService(flags types.CliFlags, manifest *types.BoxManifest, service types.Service) error {
	var projectInfo *types.ArtifactInfo
	if service.Strategy.Download == "github" {
		projectInfo = github.GetArtifactInfo(flags.Platform, flags.Architecture, manifest.Release, service)
	} else if service.Strategy.Download == "bucket" {
		projectInfo = bucket.GetArtifactInfo(flags.Platform, flags.Architecture, manifest.Release, service)
	}
	glog.Infof("Will download to %q", flags.DownloadPath)

	// Download archive
	archivePath := filepath.Join(flags.DownloadPath, projectInfo.ArchiveFileName)
	err := utils.DownloadFile(archivePath, projectInfo.ArchiveURL, flags.SkipDownloaded)
	if err != nil {
		return nil
	}

	// Download signature
	if !service.SkipGPG {
		glog.Infof("Doing GPG verification for service=%s", service.Name)
		signaturePath := filepath.Join(flags.DownloadPath, projectInfo.SignatureFileName)
		err = utils.DownloadFile(signaturePath, projectInfo.SignatureURL, flags.SkipDownloaded)
		if err != nil {
			return nil
		}
		err = verification.VerifyGPGSignature(archivePath, signaturePath)
		if err != nil {
			return nil
		}
	}

	// Download checksum
	if !service.SkipChecksum {
		glog.Infof("Doing SHA checksum verification for service=%s", service.Name)
		checksumPath := filepath.Join(flags.DownloadPath, projectInfo.ChecksumFileName)
		err = utils.DownloadFile(checksumPath, projectInfo.ChecksumURL, flags.SkipDownloaded)
		if err != nil {
			return nil
		}
		err = verification.VerifySHA256Digest(flags.DownloadPath, projectInfo.ChecksumFileName)
		if err != nil {
			return nil
		}
	}

	glog.Infof("Downloaded %s. Getting ready for extraction!", projectInfo.ArchiveFileName)
	if projectInfo.Platform == "windows" {
		glog.Info("Extracting zip archive!")
		ExtractZipArchive(archivePath, flags.DownloadPath, service)
	} else {
		glog.Info("Extracting tarball archive!")
		ExtractTarGzipArchive(archivePath, flags.DownloadPath, service)
	}
	return nil
}

func ParseYamlManifest(manifestPath string) (*types.BoxManifest, error) {
	var manifestConfig types.BoxManifest
	glog.Infof("Reading manifest file=%q", manifestPath)
	file, _ := ioutil.ReadFile(manifestPath)
	err := yaml.Unmarshal(file, &manifestConfig)
	if err != nil {
		return nil, err
	}
	if manifestConfig.Version != "3.0" {
		panic(errors.New("Invalid manifest version. Currently supported versions: 3.0"))
	}
	return &manifestConfig, nil
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
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if strings.HasSuffix(header.Name, service.ArchivePath) {
			if outputPath == "" {
				outputPath = filepath.Join(extractPath, header.Name)
			}
			glog.Infof("Extracting to %q", outputPath)
			outfile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outfile, tarReader); err != nil {
				glog.Errorf("Failed to create file: %q", outputPath)
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
	manifest, err := ParseYamlManifest(cliFlags.ManifestFile)
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
			DownloadService(cliFlags, manifest, element)
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
