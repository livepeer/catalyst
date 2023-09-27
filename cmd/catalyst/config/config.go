package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/icza/dyno"
	"gopkg.in/yaml.v2"
)

// takes /path1:/path2:/path3 and returns JSON bytes
func HandleConfigStack(configPaths string) ([]byte, error) {
	var err error
	merged := map[string]any{}
	filePaths := strings.Split(configPaths, ":")
	for _, filePath := range filePaths {
		contents, err := readYAMLFile(filePath)
		// todo: handle missing file case (allowed as long as we have some)
		if err != nil {
			return []byte{}, fmt.Errorf("error handling config file %s: %w", filePath, err)
		}
		merged = mergeMaps(merged, contents)
	}
	config, err := optionalMap(merged, "config")
	if err != nil {
		return nil, err
	}
	protocols, err := optionalMap(config, "protocols")
	if err != nil {
		return nil, err
	}
	protocolArray := []map[string]any{}
	for k, v := range protocols {
		if v == nil {
			continue
		}
		vMap, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unable to convert protocol '%s' to a string map", k)
		}
		protocolArray = append(protocolArray, vMap)
	}
	config["protocols"] = protocolArray
	jsonBytes, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

// Returns a new map merging source into dest
// Merges any map[string]any maps that are present
// Overwrites everything else
func mergeMaps(dest, source map[string]any) map[string]any {
	merged := map[string]any{}
	// Start with a shallow copy of `dest`
	for k, v := range dest {
		merged[k] = v
	}
	for newKey, newValue := range source {
		oldValue, has := merged[newKey]
		if !has {
			merged[newKey] = newValue
			continue
		}
		newMap, newOk := newValue.(map[string]any)
		oldMap, oldOk := oldValue.(map[string]any)
		if newOk && oldOk {
			// Both maps. Merge em!
			merged[newKey] = mergeMaps(oldMap, newMap)
			continue
		}
		// One or both is not a map, just copy over the new value
		merged[newKey] = newValue
	}
	return merged
}

func readYAMLFile(filePath string) (map[string]any, error) {
	var conf map[any]any
	dat, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(dat, &conf)
	if err != nil {
		return nil, err
	}
	jsonConf := dyno.ConvertMapI2MapS(conf)
	jsonMap, ok := jsonConf.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unable to convert config to a string map")
	}
	return jsonMap, nil
}

func optionalMap(parent map[string]any, key string) (map[string]any, error) {
	child, ok := parent[key]
	if !ok {
		child = map[string]any{}
		parent[key] = child
	}
	childMap, ok := child.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unable to convert '%s' to a string map", key)
	}
	return childMap, nil
}

func handleConfigFile(configPath string) (string, error) {
	var conf map[any]any
	dat, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	err = yaml.Unmarshal(dat, &conf)
	if err != nil {
		return "", err
	}
	jsonConf := dyno.ConvertMapI2MapS(conf)
	jsonMap, ok := jsonConf.(map[string]any)
	if !ok {
		return "", fmt.Errorf("unable to convert config to a string map")
	}
	config, err := optionalMap(jsonMap, "config")
	if err != nil {
		return "", err
	}
	protocols, err := optionalMap(config, "protocols")
	if err != nil {
		return "", err
	}
	protocolArray := []map[string]any{}
	for k, v := range protocols {
		vMap, ok := v.(map[string]any)
		if !ok {
			return "", fmt.Errorf("unable to convert protocol '%s' to a string map", k)
		}
		protocolArray = append(protocolArray, vMap)
	}
	config["protocols"] = protocolArray
	str, err := json.MarshalIndent(jsonConf, "", "  ")
	if err != nil {
		return "", err
	}
	return string(str), nil
}
