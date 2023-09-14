package e2e

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	advertisePort           = "9935"
	catalystAPIPort         = "7979"
	catalystAPIInternalPort = "7878"
	boxPort                 = "8888"
)

type account struct {
	Password string `json:"password"`
}

type bandwidth struct {
	Exceptions []string `json:"exceptions"`
	Limit      int      `json:"limit"`
}

type protocol struct {
	Connector        string `json:"connector"`
	RetryJoin        string `json:"retry-join,omitempty"`
	Advertise        string `json:"advertise,omitempty"`
	RPCAddr          string `json:"rpc-addr,omitempty"`
	RedirectPrefixes string `json:"redirect-prefixes,omitempty"`
	Debug            string `json:"debug,omitempty"`
	HTTPAddr         string `json:"http-addr,omitempty"`
	HTTPAddrInternal string `json:"http-internal-addr,omitempty"`
	Broadcaster      bool   `json:"broadcaster,omitempty"`
	Orchestrator     bool   `json:"orchestrator,omitempty"`
	Transcoder       bool   `json:"transcoder,omitempty"`
	HTTPRPCAddr      string `json:"httpAddr,omitempty"`
	OrchAddr         string `json:"orchAddr,omitempty"`
	ServiceAddr      string `json:"serviceAddr,omitempty"`
	CliAddr          string `json:"cliAddr,omitempty"`
	RtmpAddr         string `json:"rtmpAddr,omitempty"`
	SourceOutput     string `json:"source-output,omitempty"`
}

type config struct {
	Accesslog  string `json:"accesslog"`
	Controller struct {
		Interface interface{} `json:"interface"`
		Port      interface{} `json:"port"`
		Username  interface{} `json:"username"`
	} `json:"controller"`
	Debug         interface{} `json:"debug"`
	DefaultStream interface{} `json:"defaultStream"`
	Limits        interface{} `json:"limits"`
	Location      struct {
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
		Name string  `json:"name"`
	} `json:"location"`
	Prometheus             string               `json:"prometheus"`
	Protocols              []protocol           `json:"protocols"`
	ServerID               interface{}          `json:"serverid"`
	SessionInputMode       string               `json:"sessionInputMode"`
	SessionOutputMode      string               `json:"sessionOutputMode"`
	SessionStreamInfoMode  string               `json:"sessionStreamInfoMode"`
	SessionUnspecifiedMode string               `json:"sessionUnspecifiedMode"`
	SessionViewerMode      string               `json:"sessionViewerMode"`
	SidMode                string               `json:"sidMode"`
	Triggers               map[string][]trigger `json:"triggers"`
	Trustedproxy           []string             `json:"trustedproxy"`
}

type stream struct {
	Name         string    `json:"name"`
	Processes    []process `json:"processes,omitempty"`
	Realtime     bool      `json:"realtime"`
	Source       string    `json:"source"`
	StopSessions bool      `json:"stop_sessions"`
}

type process struct {
	Debug                 int             `json:"debug"`
	HardcodedBroadcasters string          `json:"hardcoded_broadcasters"`
	Leastlive             string          `json:"leastlive"`
	Process               string          `json:"process"`
	TargetProfiles        []targetProfile `json:"target_profiles"`
}

type targetProfile struct {
	Bitrate  int    `json:"bitrate"`
	Fps      int    `json:"fps"`
	Height   int    `json:"height"`
	Name     string `json:"name"`
	Width    int    `json:"width"`
	XLSPName string `json:"x-LSP-name"`
}

type trigger struct {
	Handler string   `json:"handler"`
	Sync    bool     `json:"sync"`
	Default string   `json:"default"`
	Streams []string `json:"streams"`
}

type mistConfig struct {
	Account      map[string]account `json:"account"`
	Autopushes   interface{}        `json:"autopushes"`
	Bandwidth    bandwidth          `json:"bandwidth"`
	Config       config             `json:"config"`
	PushSettings struct {
		Maxspeed interface{} `json:"maxspeed"`
		Wait     interface{} `json:"wait"`
	} `json:"push_settings"`
	Streams    map[string]stream `json:"streams"`
	UISettings interface{}       `json:"ui_settings"`
}

func defaultMistConfig(host, sourceOutput string) mistConfig {
	return mistConfig{
		Account: map[string]account{
			"test": {
				Password: "098f6bcd4621d373cade4e832627b4f6",
			},
		},
		Bandwidth: bandwidth{
			Exceptions: []string{},
		},
		Config: config{
			Accesslog:  "LOG",
			Prometheus: "koekjes",
			Protocols: []protocol{
				{Connector: "AAC"},
				{Connector: "CMAF"},
				{Connector: "DTSC"},
				{Connector: "EBML"},
				{Connector: "FLV"},
				{Connector: "H264"},
				{Connector: "HDS"},
				{Connector: "HLS"},
				{Connector: "HTTP"},
				{Connector: "HTTPTS"},
				{Connector: "JSON"},
				{Connector: "MP3"},
				{Connector: "MP4"},
				{Connector: "OGG"},
				{Connector: "RTMP"},
				{Connector: "RTSP"},
				{Connector: "SDP"},
				{Connector: "SRT"},
				{Connector: "TSSRT"},
				{Connector: "WAV"},
				{Connector: "WebRTC"},
				{
					Connector:   "livepeer",
					Broadcaster: true,
					CliAddr:     "127.0.0.1:7935",
					HTTPRPCAddr: "127.0.0.1:8935",
					OrchAddr:    "127.0.0.1:8936",
					RtmpAddr:    "127.0.0.1:1936",
				},
				{
					Connector:    "livepeer",
					Orchestrator: true,
					Transcoder:   true,
					CliAddr:      "127.0.0.1:7936",
					ServiceAddr:  "127.0.0.1:8936",
				},
				{
					Connector:        "livepeer-catalyst-api",
					SourceOutput:     sourceOutput,
					Advertise:        fmt.Sprintf("%s:%s", host, advertisePort),
					HTTPAddr:         fmt.Sprintf("0.0.0.0:%s", catalystAPIPort),
					HTTPAddrInternal: fmt.Sprintf("0.0.0.0:%s", catalystAPIInternalPort),
					RedirectPrefixes: "stream",
					Debug:            "6",
				},
			},
			SessionInputMode:       "14",
			SessionOutputMode:      "14",
			SessionStreamInfoMode:  "1",
			SessionUnspecifiedMode: "0",
			SessionViewerMode:      "14",
			SidMode:                "0",
			Trustedproxy:           []string{},
			Triggers: map[string][]trigger{
				"STREAM_SOURCE": {
					{
						Handler: "http://127.0.0.1:7878/STREAM_SOURCE",
						Sync:    true,
						Default: "push://",
						Streams: []string{},
					},
				},
				"PUSH_END": {
					{
						Handler: "http://127.0.0.1:7878/api/mist/trigger",
						Sync:    false,
					},
				},
				"RECORDING_END": {
					{
						Handler: "http://127.0.0.1:7878/api/mist/trigger",
						Sync:    false,
					},
				},
			},
		},
		Streams: map[string]stream{
			"stream": {
				Name:         "stream",
				Realtime:     false,
				Source:       "push://",
				StopSessions: false,
			},
		},
	}
}

func defaultMistConfigWithLivepeerProcess(host, sourceOutput string) mistConfig {
	mc := defaultMistConfig(host, sourceOutput)
	s := mc.Streams["stream"]
	s.Processes = []process{
		{Debug: 5,
			HardcodedBroadcasters: "[{\"address\":\"http://127.0.0.1:8935\"}]",
			Leastlive:             "1",
			Process:               "livepeer",
			TargetProfiles: []targetProfile{
				{Bitrate: 400000,
					Fps:      30,
					Height:   144,
					Name:     "P144p30fps16x9",
					Width:    256,
					XLSPName: "",
				},
			},
		},
	}
	mc.Streams["stream"] = s
	return mc
}

func (m *mistConfig) string() (string, error) {
	s, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(s), nil
}

func (m *mistConfig) toFile(file *os.File) error {
	str, err := m.string()
	if err != nil {
		return err
	}
	if _, err = file.Write([]byte(str)); err != nil {
		return err
	}
	return nil
}

func (m *mistConfig) toTmpFile(dir string) (string, error) {
	tmpFile, err := ioutil.TempFile(dir, "mist-config-*.json")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	err = m.toFile(tmpFile)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	return tmpFile.Name(), nil
}
