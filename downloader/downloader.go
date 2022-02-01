package downloader

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"

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
	Manifest     string
}

type Service struct {
	Name    string `yaml:"name"`
	Src     string `yaml:"src"`
	Binary  string `yaml:"binary,omitempty"`
	Release string `yaml:"release,omitempty"`
}

type manifest struct {
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
		"linux":   ".tar.gz",
		"darwin":  ".tar.gz",
		"windows": ".zip",
	}
	return platformExtMap[platform]
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func CreateDirectory(path string) bool {
	err := os.Mkdir(path, 0755)
	checkError(err)
	return true
}

func DownloadFile(path, url string) error {
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

func ParseYaml() {
	var manifestConfig manifest
	file, _ := ioutil.ReadFile("manifest.yaml")
	err := yaml.Unmarshal(file, &manifestConfig)
	if err != nil {
		fmt.Println("DAMN!")
	}
	if manifestConfig.Version != "1.0" {
		panic(errors.New("Invalid manifest version. Currently supported versions: 1.0"))
	}
	for _, element := range manifestConfig.Box {
		fmt.Println(element.Name)
	}
}

func ValidateFlags(flags CliFlags) error {
	if !IsSupportedPlatformArch(flags.Platform, flags.Architecture) {
		fmt.Println(flags)
		return errors.New(fmt.Sprintf("Invalid combination of platform+architecture detected: %s+%s", flags.Platform, flags.Architecture))
	}
	if !IsManifestFileExists(flags.Manifest) {
		return errors.New("Invalid path to manifest file!")
	}
	return nil
}

func Run(buildFlags BuildFlags) {
	cliFlags := CliFlags{}
	fs := flag.NewFlagSet("box-livepeer", flag.ExitOnError)

	fs.StringVar(&cliFlags.Platform, "platform", runtime.GOOS, "One of linux/windows/darwin")
	fs.StringVar(&cliFlags.Architecture, "architecture", runtime.GOARCH, "System architecture (amd64/arm64)")
	fs.StringVar(&cliFlags.DownloadPath, "path", fmt.Sprintf(".%sbin", string(os.PathSeparator)), "Path to store binaries")
	fs.StringVar(&cliFlags.Manifest, "manifest", "manifest.yaml", "Path to manifest yaml file")

	err := ValidateFlags(cliFlags)
	if err != nil {
		panic(err)
	}

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("LP"),
		ff.WithEnvVarSplit(","),
	)
	fmt.Println(cliFlags)
}
