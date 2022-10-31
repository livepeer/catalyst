package e2e

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	serfPort        = "7373"
	advertisePort   = "9935"
	catalystApiPort = "7979"
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
	Port             string `json:"port,omitempty"`
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
	Name         string   `json:"name"`
	Processes    []string `json:"processes"`
	Realtime     bool     `json:"realtime"`
	Source       string   `json:"source"`
	StopSessions bool     `json:"stop_sessions"`
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

func defaultMistConfig(host string) mistConfig {
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
					Connector:        "livepeer-catalyst-node",
					Advertise:        fmt.Sprintf("%s:%s", host, advertisePort),
					RPCAddr:          fmt.Sprintf("0.0.0.0:%s", serfPort),
					RedirectPrefixes: "stream",
					Debug:            "6",
				},
				{
					Connector: "livepeer-catalyst-api",
					Port:      catalystApiPort,
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
				"STREAM_SOURCE": []trigger{
					{
						Handler: "http://localhost:8091/STREAM_SOURCE",
						Sync:    true,
						Default: "push://",
						Streams: []string{},
					},
				},
			},
		},
		Streams: map[string]stream{
			"stream": {
				Name:         "stream",
				Processes:    []string{},
				Realtime:     false,
				Source:       "push://",
				StopSessions: false,
			},
		},
	}
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
