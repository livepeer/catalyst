package types

type TagInformation struct {
	Name       string `json:"name"`
	ID         uint   `json:"id"`
	PreRelease bool   `json:"prerelease"`
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Commit     string `json:"commit,omitempty"`
}

type BuildManifestInformation struct {
	Builds       map[string]string `json:"builds"`
	Commit       string            `json:"commit"`
	Branch       string            `json:"branch"`
	Ref          string            `json:"ref"`
	SrcFilenames map[string]string `json:"srcFilenames,omitempty"`
}

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

	ManifestURL bool
}

type DownloadStrategy struct {
	Download string `yaml:"download,omitempty"`
	Project  string `yaml:"project"`
	Commit   string `yaml:"commit,omitempty"`
}

type Service struct {
	Name         string            `yaml:"name"`
	Strategy     *DownloadStrategy `yaml:"strategy"`
	Binary       string            `yaml:"binary,omitempty"`
	Release      string            `yaml:"release"`
	ArchivePath  string            `yaml:"archivePath,omitempty"`
	Skip         bool              `yaml:"skip,omitempty"`
	SkipGPG      bool              `yaml:"skipGpg,omitempty"`
	SkipChecksum bool              `yaml:"skipChecksum,omitempty"`
	SrcFilenames map[string]string `yaml:"srcFilenames,omitempty"`
	OutputPath   string            `yaml:"outputPath,omitempty"`
}

type BoxManifest struct {
	Version string     `yaml:"version"`
	Release string     `yaml:"release,omitempty"`
	Box     []*Service `yaml:"box,omitempty"`
}

type ArtifactInfo struct {
	Name              string `json:"name"`
	Binary            string `json:"binary"`
	Version           string `json:"version"`
	Platform          string
	Architecture      string
	ArchiveURL        string
	ArchiveFileName   string
	ChecksumURL       string
	ChecksumFileName  string
	SignatureURL      string
	SignatureFileName string
}
