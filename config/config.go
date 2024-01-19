package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/livepeer/catalyst/test/e2e"
)

//go:embed full-stack.json
var fullstack []byte

var adminId = "00000000-0000-4000-0000-000000000000"
var recordingBucketId = "00000000-0000-4000-0000-000000000001"
var vodBucketId = "00000000-0000-4000-0000-000000000002"
var vodBucketCatalystId = "00000000-0000-4000-0000-000000000003"
var privateBucketId = "00000000-0000-4000-0000-000000000004"

type Cli struct {
	PublicURL  string
	Secret     string
	Verbosity  string
	ConfOutput string
}

func Config(cli *Cli) ([]byte, error) {
	if cli.Secret == "" {
		return []byte{}, fmt.Errorf("CATALYST_SECRET parameter is required")
	}
	u, err := url.Parse(cli.PublicURL)
	if err != nil {
		return []byte{}, err
	}
	var conf e2e.MistConfig
	err = json.Unmarshal(fullstack, &conf)
	if err != nil {
		return []byte{}, err
	}

	ret := []map[string]any{}

	admin := map[string]any{
		"id":              adminId,
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
	apiToken := map[string]any{
		"name":      "ROOT KEY DON'T DELETE",
		"createdAt": 0,
		"id":        cli.Secret,
		"kind":      "api-token",
		"userId":    admin["id"],
	}
	ret = append(ret, admin, apiToken)

	recordingBucket := ObjectStore(adminId, cli.PublicURL, recordingBucketId, "os-recordings")

	vodBucket := ObjectStore(adminId, cli.PublicURL, vodBucketId, "os-vod")

	vodBucketCatalyst := ObjectStore(adminId, cli.PublicURL, vodBucketCatalystId, "os-catalyst-vod")

	privateBucket := ObjectStore(adminId, cli.PublicURL, privateBucketId, "os-vod")
	ret = append(ret, recordingBucket, vodBucket, vodBucketCatalyst, privateBucket)

	for _, protocol := range conf.Config.Protocols {
		if protocol.Connector == "livepeer-api" && !protocol.StreamInfoService {
			protocol.RecordCatalystObjectStoreId = recordingBucketId
			protocol.VODCatalystObjectStoreId = vodBucketCatalystId
			protocol.VODCatalystPrivateAssetsObjectStore = privateBucketId
			protocol.VODObjectStoreId = vodBucketId
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
		} else if protocol.Connector == "livepeer" && protocol.Broadcaster && protocol.MetadataQueueUri != "" {
			protocol.AuthWebhookURL = fmt.Sprintf("http://%s:%s@127.0.0.1:3004/api/stream/hook", adminId, cli.Secret)
		}
	}

	video := conf.Streams["video"]
	for _, process := range video.Processes {
		if process.Process == "Livepeer" {
			process.AccessToken = cli.Secret
		}
	}

	var out []byte
	out, err = json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return []byte{}, err
	}

	return out, nil
}

func ObjectStore(id, publicUrl, userId, bucket string) map[string]any {
	return map[string]any{
		"createdAt": 0,
		"id":        "00000000-0000-4000-0000-000000000000",
		"publicUrl": "http://127.0.0.1:8888/os-private",
		"url":       "s3+http://admin:password@127.0.0.1:9000/os-private",
		"userId":    "9c2936b5-143f-4b10-b302-6a21b5f29c3d",
	}
}
