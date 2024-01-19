package config

import (
	_ "embed"
	"encoding/json"

	"github.com/livepeer/catalyst/test/e2e"
)

//go:embed full-stack.json
var fullstack []byte

func Config() ([]byte, error) {
	var conf e2e.MistConfig
	err := json.Unmarshal(fullstack, &conf)
	if err != nil {
		return []byte{}, err
	}
	var out []byte
	out, err = json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return out, nil
}
