package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/icza/dyno"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func randPath(t *testing.T) string {
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	return filepath.Join(t.TempDir(), hex.EncodeToString(randBytes)+".yaml")
}

func toFiles(t *testing.T, strs ...string) string {
	paths := []string{}
	for _, content := range strs {
		filepath := randPath(t)
		os.WriteFile(filepath, []byte(content), 0644)
		paths = append(paths, filepath)
	}
	return strings.Join(paths, ":")
}

func yamlToJson(t *testing.T, yamlStr string) string {
	var yamlStruct map[any]any
	err := yaml.Unmarshal([]byte(yamlStr), &yamlStruct)
	require.NoError(t, err)
	jsonStruct := dyno.ConvertMapI2MapS(yamlStruct)
	jsonBytes, err := json.Marshal(jsonStruct)
	require.NoError(t, err)
	return string(jsonBytes)
}

func TestMerge(t *testing.T) {
	confStack := toFiles(t, conf1, conf2, conf3)
	jsonBytes, err := HandleConfigStack(confStack)
	require.NoError(t, err)
	require.JSONEq(t, yamlToJson(t, mergedConf), string(jsonBytes))
}

var conf1 = `
foo: conf1
some-map:
  opt1: cool
config:
  protocols:
    example-protocol:
      protocol-number: 15
      protocol-boolean: true
      protocol-string: foobar
    removed-protocol:
      connector: asdf
`

var conf2 = `
foo: conf2
`

var conf3 = `
foo: conf3
some-map:
  opt2: lmao
config:
  protocols:
    example-protocol:
      protocol-string: override
    removed-protocol: null
`

var mergedConf = `
foo: conf3
some-map:
  opt1: cool
  opt2: lmao
config:
  protocols:
    - protocol-number: 15
      protocol-boolean: true
      protocol-string: override
`
