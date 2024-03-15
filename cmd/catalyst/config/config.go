package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/doug-martin/goqu/v9"
)

//go:embed full-stack.json
var fullstack []byte

//go:embed full-stack.sql
var sqlTables string

var adminID = "00000000-0000-4000-0000-000000000000"
var recordingBucketID = "00000000-0000-4000-0000-000000000001"
var vodBucketID = "00000000-0000-4000-0000-000000000002"
var vodBucketCatalystID = "00000000-0000-4000-0000-000000000003"
var privateBucketID = "00000000-0000-4000-0000-000000000004"

type Cli struct {
	PublicURL       string
	Secret          string
	Verbosity       string
	ConfOutput      string
	SQLOutput       string
	Network         string
	EthURL          string
	EthKeystorePath string
	EthPassword     string
}

type DBObject map[string]any

func (d DBObject) Table() string {
	switch d["kind"] {
	case "user":
		return "users"
	case "api-token":
		return "api_token"
	case "object-store":
		return "object_store"
	}
	panic("table not found")
}

func GenerateConfig(cli *Cli) ([]byte, []byte, error) {
	if cli.Secret == "" {
		return []byte{}, []byte{}, fmt.Errorf("CATALYST_SECRET parameter is required")
	}
	u, err := url.Parse(cli.PublicURL)
	if err != nil {
		return []byte{}, []byte{}, err
	}
	var conf MistConfig
	err = json.Unmarshal(fullstack, &conf)
	if err != nil {
		return []byte{}, []byte{}, err
	}

	inserts := []DBObject{}

	admin := DBObject{
		"id":              adminID,
		"firstName":       "Root",
		"lastName":        "User",
		"admin":           true,
		"createdAt":       0,
		"email":           "admin@example.com",
		"emailValid":      true,
		"emailValidToken": "00000000-0000-4000-0000-000000000000",
		"kind":            "user",
		"lastSeen":        1694546853946,
		"password":        "0000000000000000000000000000000000000000000000000000000000000000",
		"salt":            "0000000000000000",
	}
	apiToken := DBObject{
		"name":      "ROOT KEY DON'T DELETE",
		"createdAt": 0,
		"id":        cli.Secret,
		"kind":      "api-token",
		"userId":    admin["id"],
	}
	inserts = append(inserts, admin, apiToken)

	recordingBucket := ObjectStore(adminID, cli.PublicURL, recordingBucketID, "os-recordings")

	vodBucket := ObjectStore(adminID, cli.PublicURL, vodBucketID, "os-vod")

	vodBucketCatalyst := ObjectStore(adminID, cli.PublicURL, vodBucketCatalystID, "os-catalyst-vod")

	privateBucket := ObjectStore(adminID, cli.PublicURL, privateBucketID, "os-vod")
	inserts = append(inserts, recordingBucket, vodBucket, vodBucketCatalyst, privateBucket)

	newProtocols := []*Protocol{}
	for _, protocol := range conf.Config.Protocols {
		ok := tweakProtocol(protocol, cli, u)
		if ok {
			newProtocols = append(newProtocols, protocol)
		}
	}
	conf.Config.Protocols = newProtocols

	video := conf.Streams["video"]
	for _, process := range video.Processes {
		if process.Process == "Livepeer" {
			process.AccessToken = cli.Secret
		}
	}

	var out []byte
	out, err = json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return []byte{}, []byte{}, err
	}

	sql := strings.ReplaceAll(sqlTables, "CREATE TABLE", "CREATE TABLE IF NOT EXISTS")

	for _, insert := range inserts {
		obj, err := json.Marshal(insert)
		if err != nil {
			return []byte{}, []byte{}, err
		}
		ds := goqu.Insert(insert.Table()).Rows(
			goqu.Record{"id": insert["id"], "data": obj},
		).OnConflict(goqu.DoNothing())
		insertSQL, _, err := ds.ToSQL()
		if err != nil {
			return []byte{}, []byte{}, err
		}

		sql = fmt.Sprintf("%s\n%s;", sql, insertSQL)
	}

	return out, []byte(sql), nil
}

// returns true if this protocol should be included
func tweakProtocol(protocol *Protocol, cli *Cli, u *url.URL) bool {
	if protocol.Connector == "livepeer-api" && !protocol.StreamInfoService {
		protocol.RecordCatalystObjectStoreID = recordingBucketID
		protocol.VODCatalystObjectStoreID = vodBucketCatalystID
		protocol.VODCatalystPrivateAssetsObjectStore = privateBucketID
		protocol.VODObjectStoreID = vodBucketID
		protocol.CORSJWTAllowlist = fmt.Sprintf(`["%s"]`, cli.PublicURL)
		protocol.Ingest = fmt.Sprintf(
			`[{"ingest":"rtmp://%s/live","ingests":{"rtmp":"rtmp://%s/live","srt":"srt://%s:8889"},"playback":"%s/mist/hls","base":"%s","origin":"%s"}]`,
			u.Hostname(),
			u.Hostname(),
			u.Hostname(),
			cli.PublicURL,
			cli.PublicURL,
			cli.PublicURL,
		)
	} else if protocol.Connector == "livepeer-catalyst-api" {
		protocol.APIToken = cli.Secret
		protocol.Tags = fmt.Sprintf("node=media,http=%s/mist,https=%s/mist", cli.PublicURL, cli.PublicURL)
	} else if protocol.Connector == "livepeer-task-runner" {
		protocol.CatalystSecret = cli.Secret
		protocol.LivepeerAccessToken = cli.Secret
	} else if protocol.Connector == "livepeer-analyzer" {
		protocol.LivepeerAccessToken = cli.Secret
	} else if protocol.Connector == "livepeer" && protocol.Broadcaster {
		// both broadcasters
		if cli.Network != "offchain" {
			protocol.Network = cli.Network
			protocol.EthKeystorePath = cli.EthKeystorePath
			protocol.EthPassword = cli.EthPassword
			protocol.EthURL = cli.EthURL
		}
	} else if protocol.Connector == "livepeer" && protocol.Broadcaster && protocol.MetadataQueueURI != "" {
		// live broadcaster
		protocol.AuthWebhookURL = fmt.Sprintf("http://%s:%s@127.0.0.1:3004/api/stream/hook", adminID, cli.Secret)
	} else if protocol.Connector == "livepeer" && protocol.Orchestrator {
		// if we're not offchain we shouldn't run a local O
		if cli.Network != "offchain" {
			return false
		}
	} else if protocol.Connector == "WebRTC" {
		protocol.ICEServers = []ICEServer{
			{
				URLs: fmt.Sprintf("stun:%s:3478", u.Hostname()),
			},
			{
				Credential: "livepeer",
				URLs:       fmt.Sprintf("turn:%s:3478", u.Hostname()),
				Username:   "livepeer",
			},
			{
				URLs: fmt.Sprintf("stun:%s:5349", u.Hostname()),
			},
			{
				Credential: "livepeer",
				URLs:       fmt.Sprintf("turn:%s:5349", u.Hostname()),
				Username:   "livepeer",
			},
		}
	}
	return true
}

func ObjectStore(userID, publicURL, id, bucket string) DBObject {
	return DBObject{
		"createdAt": 0,
		"id":        id,
		"publicUrl": fmt.Sprintf("%s/%s", publicURL, bucket),
		"url":       fmt.Sprintf("s3+http://admin:password@127.0.0.1:9000/%s", bucket),
		"userId":    userID,
		"kind":      "object-store",
	}
}
