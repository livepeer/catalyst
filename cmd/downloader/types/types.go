package types

type gitRefObject struct {
	SHA  string `json:"sha"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

type GitRefInfo struct {
	Object gitRefObject `json:"object"`
	URL    string       `json:"url"`
	Ref    string       `json:"ref"`
}

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
	UpdateManifest bool
	Download       bool
	DownloadPath   string
	Platform       string
	Architecture   string
	ManifestFile   string
	Verbosity      string

	ManifestURL    bool
	MistController string
	ConfigStack    string
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

	SkipManifestUpdate bool `yaml:"skipManifestUpdate,omitempty"`
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
