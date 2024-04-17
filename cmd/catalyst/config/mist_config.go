package config

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

type Account struct {
	Password string `json:"password"`
}

type Bandwidth struct {
	Exceptions []string `json:"exceptions"`
	Limit      int      `json:"limit"`
}

type ICEServer struct {
	URLs       string `json:"urls,omitempty"`
	Username   string `json:"username,omitempty"`
	Credential string `json:"credential,omitempty"`
}
type Protocol struct {
	Connector                           string `json:"connector"`
	RetryJoin                           string `json:"retry-join,omitempty"`
	Advertise                           string `json:"advertise,omitempty"`
	RPCAddr                             string `json:"rpc-addr,omitempty"`
	RedirectPrefixes                    string `json:"redirect-prefixes,omitempty"`
	Debug                               string `json:"debug,omitempty"`
	HTTPAddr                            string `json:"http-addr,omitempty"`
	HTTPAddrInternal                    string `json:"http-internal-addr,omitempty"`
	Broadcaster                         bool   `json:"broadcaster,omitempty"`
	Orchestrator                        bool   `json:"orchestrator,omitempty"`
	Transcoder                          bool   `json:"transcoder,omitempty"`
	HTTPRPCAddr                         string `json:"httpAddr,omitempty"`
	OrchAddr                            string `json:"orchAddr,omitempty"`
	ServiceAddr                         string `json:"serviceAddr,omitempty"`
	CliAddr                             string `json:"cliAddr,omitempty"`
	RtmpAddr                            string `json:"rtmpAddr,omitempty"`
	SourceOutput                        string `json:"source-output,omitempty"`
	Catabalancer                        string `json:"catabalancer,omitempty"`
	BaseStreamName                      string `json:"base-stream-name,omitempty"`
	Broadcasters                        string `json:"broadcasters,omitempty"`
	JWTAudience                         string `json:"jwt-audience,omitempty"`
	JWTSecret                           string `json:"jwt-secret,omitempty"`
	OwnRegion                           string `json:"own-region,omitempty"`
	PostgresURL                         string `json:"postgres-url,omitempty"`
	RecordCatalystObjectStoreID         string `json:"recordCatalystObjectStoreId,omitempty"`
	VODCatalystObjectStoreID            string `json:"vodCatalystObjectStoreId,omitempty"`
	VODCatalystPrivateAssetsObjectStore string `json:"vodCatalystPrivateAssetsObjectStore,omitempty"`
	VODObjectStoreID                    string `json:"vodObjectStoreId,omitempty"`
	Port                                string `json:"port,omitempty"`
	StreamInfoService                   bool   `json:"stream-info-service,omitempty"`
	V                                   string `json:"v,omitempty"`
	OwnBaseURL                          string `json:"own-base-url,omitempty"`
	Node                                string `json:"node,omitempty"`
	Monitor                             bool   `json:"monitor,omitempty"`
	MetricsPerStream                    bool   `json:"metricsPerStream,omitempty"`
	MetricsClientIP                     bool   `json:"metricsClientIP,omitempty"`
	CatalystURL                         string `json:"catalyst-url,omitempty"`
	BroadcasterURL                      string `json:"broadcaster-url,omitempty"`
	DisableBigquery                     bool   `json:"disable-bigquery,omitempty"`
	BindHost                            string `json:"bindhost,omitempty"`
	BalancerArgs                        string `json:"balancer-args,omitempty"`
	APIServer                           string `json:"api-server,omitempty"`
	APIToken                            string `json:"api-token,omitempty"`
	CatalystSecret                      string `json:"catalyst-secret,omitempty"`
	PubHost                             string `json:"pubhost,omitempty"`
	Tags                                string `json:"tags,omitempty"`
	Ingest                              string `json:"ingest,omitempty"`
	CORSJWTAllowlist                    string `json:"cors-jwt-allowlist,omitempty"`
	LivepeerAccessToken                 string `json:"livepeer-access-token,omitempty"`
	AuthWebhookURL                      string `json:"authWebhookUrl,omitempty"`
	Network                             string `json:"network,omitempty"`
	EthURL                              string `json:"ethUrl,omitempty"`
	EthKeystorePath                     string `json:"ethKeystorePath,omitempty"`
	EthPassword                         string `json:"ethPassword,omitempty"`
	MaxTicketEV                         string `json:"maxTicketEV,omitempty"`
	MaxTotalEV                          string `json:"maxTotalEV,omitempty"`
	MaxPricePerUnit                     string `json:"maxPricePerUnit,omitempty"`

	ICEServers []ICEServer `json:"iceservers,omitempty"`
	// And finally, four ways to spell the same thing:
	AMQPURL          string `json:"amqp-url,omitempty"`
	AMQPURI          string `json:"amqp-uri,omitempty"`
	MetadataQueueURI string `json:"metadataQueueUri,omitempty"`
	RabbitMQURI      string `json:"rabbitmq-uri,omitempty"`
}

type Config struct {
	Accesslog  string `json:"accesslog,omitempty"`
	Controller *struct {
		Interface *string `json:"interface,omitempty"`
		Port      *string `json:"port,omitempty"`
		Username  *string `json:"username,omitempty"`
	} `json:"controller,omitempty"`
	Debug         interface{} `json:"debug,omitempty"`
	DefaultStream string      `json:"defaultStream,omitempty"`
	Limits        interface{} `json:"limits,omitempty"`
	Location      *struct {
		Lat  float64 `json:"lat,omitempty"`
		Lon  float64 `json:"lon,omitempty"`
		Name string  `json:"name,omitempty"`
	} `json:"location,omitempty"`
	Prometheus             string               `json:"prometheus,omitempty"`
	Protocols              []*Protocol          `json:"protocols,omitempty"`
	ServerID               interface{}          `json:"serverid,omitempty"`
	SessionInputMode       int                  `json:"sessionInputMode"`
	SessionOutputMode      int                  `json:"sessionOutputMode"`
	SessionStreamInfoMode  int                  `json:"sessionStreamInfoMode"`
	SessionUnspecifiedMode int                  `json:"sessionUnspecifiedMode"`
	SessionViewerMode      int                  `json:"sessionViewerMode"`
	SidMode                int                  `json:"sidMode,omitempty"`
	TknMode                int                  `json:"tknMode,omitempty"`
	Triggers               map[string][]Trigger `json:"triggers"`
	Trustedproxy           []string             `json:"trustedproxy,omitempty"`
}

type Stream struct {
	Name         string     `json:"name"`
	Processes    []*Process `json:"processes,omitempty"`
	Realtime     bool       `json:"realtime,omitempty"`
	Source       string     `json:"source"`
	StopSessions bool       `json:"stop_sessions"`
	DVR          int        `json:"DVR"`
	MaxKeepAway  int        `json:"maxkeepaway"`
	SegmentSize  int        `json:"segmentsize"`
}

type Process struct {
	Debug                 int             `json:"debug,omitempty"`
	HardcodedBroadcasters string          `json:"hardcoded_broadcasters,omitempty"`
	Exec                  string          `json:"exec,omitempty"`
	Leastlive             bool            `json:"leastlive,omitempty"`
	Process               string          `json:"process,omitempty"`
	TargetProfiles        []TargetProfile `json:"target_profiles,omitempty"`
	AccessToken           string          `json:"access_token,omitempty"`
	CustomURL             string          `json:"custom_url,omitempty"`
	ExitUnmask            bool            `json:"exit_unmask"`
	TrackInhibit          string          `json:"track_inhibit,omitempty"`
	TrackSelect           string          `json:"track_select,omitempty"`
	XLSPName              string          `json:"x-LSP-name,omitempty"`
}

type TargetProfile struct {
	Bitrate  int    `json:"bitrate"`
	Fps      int    `json:"fps"`
	Height   int    `json:"height"`
	Name     string `json:"name"`
	Profile  string `json:"profile"`
	Width    int    `json:"width"`
	XLSPName string `json:"x-LSP-name"`
}

type Trigger struct {
	Handler string   `json:"handler"`
	Sync    bool     `json:"sync"`
	Default string   `json:"default"`
	Streams []string `json:"streams"`
}

type MistConfig struct {
	Account      map[string]Account `json:"account"`
	Autopushes   interface{}        `json:"autopushes"`
	Bandwidth    *Bandwidth         `json:"bandwidth,omitempty"`
	Config       Config             `json:"config"`
	PushSettings struct {
		Maxspeed interface{} `json:"maxspeed"`
		Wait     interface{} `json:"wait"`
	} `json:"push_settings"`
	Streams    map[string]*Stream `json:"streams"`
	UISettings interface{}        `json:"ui_settings,omitempty"`
	ExtWriters []any              `json:"extwriters"`
}

func DefaultMistConfig(host, sourceOutput string) MistConfig {
	return MistConfig{
		Account: map[string]Account{
			"test": {
				Password: "098f6bcd4621d373cade4e832627b4f6",
			},
		},
		Bandwidth: &Bandwidth{
			Exceptions: []string{},
		},
		Config: Config{
			Accesslog:  "LOG",
			Prometheus: "koekjes",
			Protocols: []*Protocol{
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
					Catabalancer:     "enabled",
				},
			},
			SessionInputMode:       14,
			SessionOutputMode:      14,
			SessionStreamInfoMode:  1,
			SessionUnspecifiedMode: 0,
			SessionViewerMode:      14,
			SidMode:                0,
			Trustedproxy:           []string{},
			Triggers: map[string][]Trigger{
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
		Streams: map[string]*Stream{
			"stream": {
				Name:         "stream",
				Realtime:     false,
				Source:       "push://",
				StopSessions: false,
			},
		},
	}
}

func DefaultMistConfigWithLivepeerProcess(host, sourceOutput string) MistConfig {
	mc := DefaultMistConfig(host, sourceOutput)
	s := mc.Streams["stream"]
	s.Processes = []*Process{
		{
			Debug:                 5,
			HardcodedBroadcasters: "[{\"address\":\"http://127.0.0.1:8935\"}]",
			Leastlive:             true,
			Process:               "livepeer",
			TargetProfiles: []TargetProfile{
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

func (m *MistConfig) string() (string, error) {
	s, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(s), nil
}

func (m *MistConfig) toFile(file *os.File) error {
	str, err := m.string()
	if err != nil {
		return err
	}
	if _, err = file.Write([]byte(str)); err != nil {
		return err
	}
	return nil
}

func (m *MistConfig) ToTmpFile(dir string) (string, error) {
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
